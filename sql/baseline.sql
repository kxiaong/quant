CREATE TABLE coinbene_kline(
    create_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    high numeric(12, 6),
    low numeric(12, 6),
    open numeric(12, 6),
    volume numeric(12, 6),
    gap varchar(8),
    ts timestamp without time zone
) PARTITION BY LIST(gap);

CREATE TABLE coinbene_kline_1m PARTITION OF coinbene_kline FOR VALUES IN ('1m');
CREATE TABLE coinbene_kline_5m PARTITION OF coinbene_kline FOR VALUES IN ('5m');
CREATE TABLE coinbene_kline_15m PARTITION OF coinbene_kline FOR VALUES IN ('15m');
CREATE TABLE coinbene_kline_30m PARTITION OF coinbene_kline FOR VALUES IN ('30m');
CREATE TABLE coinbene_kline_1h PARTITION OF coinbene_kline FOR VALUES IN ('1h');
CREATE TABLE coinbene_kline_4h PARTITION OF coinbene_kline FOR VALUES IN ('4h');
CREATE TABLE coinbene_kline_6h PARTITION OF coinbene_kline FOR VALUES IN ('6h');
CREATE TABLE coinbene_kline_12h PARTITION OF coinbene_kline FOR VALUES IN ('12h');
CREATE TABLE coinbene_kline_1d PARTITION OF coinbene_kline FOR VALUES IN ('1d');


CREATE TYPE SIDE AS ENUM(
    'buy',
    'sell'
);

CREATE TABLE coinbene_trade_list(
    id SERIAL PRIMARY KEY,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    price numeric(12, 8),
    direction  SIDE NOT NULL,
    amount integer,
    ts timestamp without time zone NOT NULL
);
