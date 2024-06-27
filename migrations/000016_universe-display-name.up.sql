alter table asset_universe add column display_name text not null default '';

create view asset_universe_size as
  select max(display_name) as "display_name", asset_universe_name, count(*) as num_assets
  from asset_universe inner join asset_universe_ticker on asset_universe.asset_universe_id = asset_universe_ticker.asset_universe_id
  group by asset_universe_name;