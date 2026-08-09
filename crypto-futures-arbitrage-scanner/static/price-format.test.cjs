const test = require('node:test');
const assert = require('node:assert/strict');

const {
    chartPriceFormat,
    formatPrice,
    precisionForPrice,
} = require('./price-format.js');

test('precisionForPrice keeps existing boundaries and uses eight decimals below one', () => {
    const cases = [
        [1000, 2],
        [999.9, 3],
        [100, 3],
        [99.9, 4],
        [10, 4],
        [9.9, 5],
        [1, 5],
        [0.99999999, 8],
        [0.01140723, 8],
    ];

    for (const [price, expected] of cases) {
        assert.equal(precisionForPrice(price), expected, `price ${price}`);
    }
});

test('formatPrice preserves all eight meaningful COTI decimals', () => {
    assert.equal(formatPrice(0.01140723), '0.01140723');
});

test('chartPriceFormat configures Lightweight Charts for eight decimals', () => {
    assert.deepEqual(chartPriceFormat(0.01140723), {
        type: 'price',
        precision: 8,
        minMove: 0.00000001,
    });
});
