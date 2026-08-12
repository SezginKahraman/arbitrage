import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { TransferRouteEvaluation } from '../../app/types';
import { ExecutionChecks } from './ExecutionChecks';

const route: TransferRouteEvaluation = {
  asset: 'COTI', source: 'gate_spot', destination: 'kucoin_spot', status: 'check',
  reason: 'common network requires verification', checkedAt: 20_000,
  networks: [
    {
      networkID: 'coti_evm', name: 'COTI', status: 'check', reason: 'network alias requires verification',
      sourceWithdrawEnabled: true, destinationDepositEnabled: true,
      withdrawalFee: '', minimumWithdrawal: '0.9827044', contractAddress: '',
    },
    {
      networkID: 'ethereum', name: 'Ethereum(ERC20)', status: 'blocked', reason: 'source withdrawal disabled',
      sourceWithdrawEnabled: false, destinationDepositEnabled: true,
      withdrawalFee: '', minimumWithdrawal: '0.9827044', contractAddress: '0xddb3422497e61e13543bea06989c0789117555c5',
    },
  ],
  sourceNetworks: [
    {
      asset: 'COTI', networkID: 'coti_evm', rawNetworkID: 'COTI', name: 'COTI', contractAddress: '',
      depositEnabled: true, withdrawEnabled: true, withdrawalFee: '', minimumWithdrawal: '0.9827044', confirmations: 0, checkedAt: 20_000,
    },
    {
      asset: 'COTI', networkID: 'ethereum', rawNetworkID: 'ETH', name: 'Ethereum(ERC20)', contractAddress: '0xddb3422497e61e13543bea06989c0789117555c5',
      depositEnabled: true, withdrawEnabled: false, withdrawalFee: '', minimumWithdrawal: '0.9827044', confirmations: 0, checkedAt: 20_000,
    },
  ],
  destinationNetworks: [
    {
      asset: 'COTI', networkID: 'coti_evm', rawNetworkID: 'cotievm', name: 'COTI', contractAddress: '',
      depositEnabled: true, withdrawEnabled: true, withdrawalFee: '150', minimumWithdrawal: '300', confirmations: 100, checkedAt: 21_000,
    },
  ],
};

describe('ExecutionChecks', () => {
  it('lists directional venue networks and their open or closed state', () => {
    render(<ExecutionChecks requestStatus="ready" route={route} />);

    expect(screen.getByText('Transfer route check required')).toBeInTheDocument();
    expect(screen.getByText('Common network requires verification')).toBeInTheDocument();
    expect(screen.getByText('Gate.io Spot networks')).toBeInTheDocument();
    expect(screen.getByText('KuCoin Spot networks')).toBeInTheDocument();
    expect(screen.getByText('Withdrawal closed')).toBeInTheDocument();
    expect(screen.getByText('Fee 150 COTI')).toBeInTheDocument();
    expect(screen.getByText('Minimum 300 COTI')).toBeInTheDocument();
  });

  it('explains sanitized Binance credential rejection', () => {
    render(<ExecutionChecks requestStatus="ready" route={{
      ...route, source: 'binance_spot', status: 'unknown', reason: 'binance_spot credentials rejected',
      networks: [], sourceNetworks: [], destinationNetworks: [],
    }} />);

    expect(screen.getByText('Transfer route unknown')).toBeInTheDocument();
    expect(screen.getByText('Binance Spot credentials rejected')).toBeInTheDocument();
  });
});
