import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('edge_paper',Path(__file__).with_name('edge_paper.py'))
e=importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)


def book(bid,ask,now=1000):
    return dict(bids=[(bid,100)],asks=[(ask,100)],timestamp=now,received=now,elapsed=.1)


def markets():
    return {v:{'TEST':dict(e.market('TESTUSDT',1,'.1',.0005,1000000,0,3600,4600),min_qty=.1)} for v in e.VENUES}


class PaperTests(unittest.TestCase):
    def test_depth_vwap_and_insufficient_liquidity(self):
        self.assertEqual(e.vwap([(10,2),(12,3)],4),11)
        with self.assertRaises(ValueError):e.vwap([(10,2)],3)

    def test_common_quantity_uses_lcm_not_maximum_step(self):
        self.assertEqual(e.common_quantity(.019,['.002','.003']),.018)
        self.assertEqual(e.common_quantity(.0009,['.0001','.001']),0)

    def test_exit_signs_and_all_four_fees(self):
        p=dict(quantity=1,long_entry=100,short_entry=104,long_fee=.001,short_fee=.002,entry_fees=.308,latency_cost=.1,funding_estimate=0)
        x=e.exit_value(p,book(102,103),book(100,101))
        self.assertAlmostEqual(x['price_pnl'],5)
        self.assertAlmostEqual(x['fees'],.612)
        self.assertAlmostEqual(x['net'],4.288)

    def test_staleness_skew_and_missing_timestamp(self):
        a,b=book(10,11),book(10,11)
        self.assertTrue(e.fresh(a,b)[0])
        b['timestamp']=0
        self.assertEqual(e.fresh(a,b)[1],'missing_exchange_timestamp')
        b=book(10,11,1004)
        self.assertEqual(e.fresh(a,b)[1],'cross_venue_skew')
        b=book(10,11);b['elapsed']=4
        self.assertEqual(e.fresh(a,b)[1],'stale_book')

    def test_decision_time_rejects_old_batch_even_when_transport_fast(self):
        self.assertEqual(e.fresh(book(10,11),book(10,11),1004)[1],'stale_book')

    def test_funding_forecast_counts_scheduled_events_and_exit_friction(self):
        state=e.new_state();ms=markets()
        for v in e.VENUES:
            ms[v]['TEST']['interval']=86400
            ms[v]['TEST']['next']=1001
        ms['gate']['TEST']['rate']=.01
        # 10% wide book overwhelms a 1% forecast, so no carry position may open.
        books={('TEST',v):book(90,100) for v in e.VENUES}
        result=e.cycle(state,ms,books,1000,100,'one')
        row=next(r for r in result['observations'] if r['long']=='binance' and r['short']=='gate')
        self.assertAlmostEqual(row['forecast_funding_24h'],.9)
        self.assertLess(row['immediate_exit_net'],-19)
        self.assertFalse(any(p['strategy']=='funding' for p in state['positions']))

    def test_funding_unknown_does_not_qualify(self):
        m=e.market('T',1,1,.001,rate=.1,next_time=2000)
        self.assertFalse(e.funding_known(m,1000))
        m['interval']=3600
        self.assertTrue(e.funding_known(m,1000))
        m['next']=500
        self.assertFalse(e.funding_known(m,1000))

    def test_new_trade_is_negative_and_cannot_close_same_snapshot_reload_idempotent(self):
        state=e.new_state();ms=markets()
        books={('TEST','binance'):book(99.9,100),('TEST','gate'):book(101,101.1),('TEST','kucoin'):book(100.4,100.5)}
        result=e.cycle(state,ms,books,1000,100,'one')
        self.assertEqual(len(state['positions']),1)
        p=state['positions'][0]
        self.assertEqual(p['long'],'binance');self.assertEqual(p['short'],'gate')
        self.assertLess(p['last_value']['net'],0)
        self.assertEqual(len(state['closed']),0)
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'state.json';e.atomic_json(path,state);loaded=e.load_state(path)
            self.assertTrue(e.cycle(loaded,ms,books,1001,100,'one')['duplicate'])
            self.assertEqual(len(loaded['positions']),1)
        # Converged next snapshot: exit price gain must include entry and exit costs.
        books={('TEST',v):book(100.49,100.5,1300) for v in e.VENUES}
        result=e.cycle(state,ms,books,1300,100,'two')
        self.assertEqual(len(state['closed']),1)
        self.assertGreater(state['closed'][0]['pnl'],0)
        self.assertAlmostEqual(sum(state['capital']['convergence'].values())-3000,state['closed'][0]['pnl'])

    def test_missing_settlement_suppresses_profit_exit(self):
        state=e.new_state();ms=markets()
        books={('TEST','binance'):book(99.9,100),('TEST','gate'):book(101,101.1),('TEST','kucoin'):book(100.4,100.5)}
        e.cycle(state,ms,books,1000,100,'one')
        p=state['positions'][0];p['next_funding']={'long':1100,'short':1100}
        books={('TEST',v):book(100.49,100.5,1300) for v in e.VENUES}
        with patch.object(e,'settled_history',return_value=[]):
            e.cycle(state,ms,books,1300,100,'two')
        self.assertEqual(len(state['closed']),0)
        self.assertTrue(p['funding_pending'])

    def test_settled_funding_deduplicates_and_signs(self):
        p=dict(long='binance',short='gate',coin='TEST',long_symbol='TEST',short_symbol='TEST',opened=1000,
            next_funding={'long':1100,'short':1100},quantity=2,funding_estimate=0,funding_events={})
        books={('TEST',v):book(99,101,1300) for v in e.VENUES}
        with patch.object(e,'settled_history',return_value=[(1100,.001,100)]):
            e.accrue(p,markets(),books,1300)
            e.accrue(p,markets(),books,1300)
        self.assertEqual(len(p['funding_events']),2)
        self.assertEqual(p['funding_estimate'],0)
        self.assertEqual(p['funding_events']['long:1100']['cashflow'],-.2)
        self.assertEqual(p['funding_events']['short:1100']['cashflow'],.2)

    def test_old_settlement_does_not_hide_later_missing_payments(self):
        p=dict(long='binance',short='gate',coin='TEST',long_symbol='TEST',short_symbol='TEST',opened=1000,
            next_funding={'long':1100,'short':1100},funding_intervals={'long':3600,'short':3600},quantity=2,funding_estimate=0,funding_events={})
        ms=markets()
        for v in e.VENUES:ms[v]['TEST']['next']=11900
        books={('TEST',v):book(99,101,8500) for v in e.VENUES}
        with patch.object(e,'settled_history',return_value=[(1100,.001,100)]):
            e.accrue(p,ms,books,8500)
        self.assertTrue(p['funding_pending'])
        self.assertEqual(p['next_funding']['long'],1100)

    def test_clock_after_blocking_funding_rejects_delayed_exit(self):
        state=e.new_state();ms=markets()
        books={('TEST','binance'):book(99.9,100),('TEST','gate'):book(101,101.1),('TEST','kucoin'):book(100.4,100.5)}
        e.cycle(state,ms,books,1000,100,'one')
        books={('TEST',v):book(100.49,100.5,1300) for v in e.VENUES}
        e.cycle(state,ms,books,1300,100,'two',decision_clock=lambda:1305)
        self.assertEqual(len(state['closed']),0)
        self.assertTrue(state['positions'][0]['valuation_stale'])

    def test_report_includes_open_losses_and_reserved_collateral(self):
        state=e.new_state();ms=markets()
        books={('TEST','binance'):book(99.9,100),('TEST','gate'):book(101,101.1),('TEST','kucoin'):book(100.4,100.5)}
        result=e.cycle(state,ms,books,1000,100,'one')
        r=e.report(state,result,1000,None,{},['TEST'],1)
        self.assertLess(r['strategies']['convergence']['modeled_equity'],3000)
        self.assertGreater(r['strategies']['convergence']['reserved_by_venue']['binance'],0)


if __name__=='__main__':unittest.main()
