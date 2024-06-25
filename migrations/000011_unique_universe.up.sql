alter table asset_universe
ADD CONSTRAINT unique_asset_universe_name UNIQUE (asset_universe_name);

alter type asset_universe_name add value 'ALL';

-- INSERT INTO asset_universe_ticker (ticker_id, asset_universe_id)
-- SELECT t.ticker_id, au.asset_universe_id
-- FROM ticker t
-- JOIN asset_universe au ON au.asset_universe_name = 'SPY_TOP_80';