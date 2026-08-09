(function (root, factory) {
    const formatter = factory();

    if (typeof module === 'object' && module.exports) {
        module.exports = formatter;
    }

    root.PriceFormatting = formatter;
}(typeof globalThis !== 'undefined' ? globalThis : this, function () {
    function asFinitePrice(price) {
        const value = Number(price);
        if (!Number.isFinite(value)) {
            throw new TypeError('price must be a finite number');
        }
        return value;
    }

    function precisionForPrice(price) {
        const value = Math.abs(asFinitePrice(price));

        if (value >= 1000) return 2;
        if (value >= 100) return 3;
        if (value >= 10) return 4;
        if (value >= 1) return 5;
        return 8;
    }

    function formatPrice(price) {
        const value = asFinitePrice(price);
        return value.toFixed(precisionForPrice(value));
    }

    function chartPriceFormat(price) {
        const precision = precisionForPrice(price);
        return {
            type: 'price',
            precision,
            minMove: 10 ** -precision,
        };
    }

    return Object.freeze({
        chartPriceFormat,
        formatPrice,
        precisionForPrice,
    });
}));
