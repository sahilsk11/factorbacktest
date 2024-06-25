alter table factor_score
add column error text;

alter table factor_score
alter column score drop not null