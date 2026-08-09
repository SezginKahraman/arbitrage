const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const PriceFormatting = require('./price-format.js');

function loadScannerClass() {
    const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');
    const context = vm.createContext({
        console,
        document: { addEventListener() {} },
        PriceFormatting,
        setTimeout,
        clearTimeout,
    });
    vm.runInContext(
        `${source}\nglobalThis.FuturesArbitrageScanner = FuturesArbitrageScanner;`,
        context,
    );
    return context.FuturesArbitrageScanner;
}

test('scanner uses the shared eight-decimal formatter for COTI prices', () => {
    const Scanner = loadScannerClass();
    const scanner = Object.create(Scanner.prototype);

    assert.equal(scanner.formatPrice(0.01140723), '0.01140723');
});

test('source updates configure chart precision once per precision band', () => {
    const Scanner = loadScannerClass();
    const scanner = Object.create(Scanner.prototype);
    const appliedOptions = [];

    scanner.sources = new Map();
    scanner.connectedSources = new Set();
    scanner.chartSeries = new Map([
        ['binance_spot', {
            applyOptions(options) {
                appliedOptions.push(options);
            },
        }],
    ]);
    scanner.sourcePricePrecisions = new Map();

    scanner.updateSourcePrice('binance_spot', 0.01140723);
    scanner.updateSourcePrice('binance_spot', 0.01140724);

    assert.equal(appliedOptions.length, 1);
    assert.equal(appliedOptions[0].priceFormat.type, 'price');
    assert.equal(appliedOptions[0].priceFormat.precision, 8);
    assert.equal(appliedOptions[0].priceFormat.minMove, 0.00000001);
});
