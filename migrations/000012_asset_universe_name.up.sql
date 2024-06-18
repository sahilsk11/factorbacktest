alter table asset_universe
rename column asset_universe_name to old_asset_universe_name;

alter table asset_universe drop constraint unique_asset_universe_name;

alter table asset_universe
add column asset_universe_name text;

update asset_universe
set asset_universe_name = old_asset_universe_name::text;


alter table asset_universe
alter column asset_universe_name set not null;

alter table asset_universe
add constraint unique_asset_universe_name unique(asset_universe_name);

alter table asset_universe
alter column asset_universe_name set not null;

alter table asset_universe
drop column old_asset_universe_name;

drop type asset_universe_name;