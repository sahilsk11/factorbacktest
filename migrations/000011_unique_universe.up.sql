alter table asset_universe
ADD CONSTRAINT unique_asset_universe_name UNIQUE (asset_universe_name);

alter type asset_universe_name add value 'ALL';

alter table asset_universe_ticker
add constraint unique_asset_in_universe unique (ticker_id, asset_universe_id);

-- INSERT INTO asset_universe_ticker (ticker_id, asset_universe_id)
-- SELECT t.ticker_id, au.asset_universe_id
-- FROM ticker t
-- JOIN asset_universe au ON au.asset_universe_name = 'SPY_TOP_80';