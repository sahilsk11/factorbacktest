update investment
set paused_at = null
where end_date is null
  and paused_at is not null;
