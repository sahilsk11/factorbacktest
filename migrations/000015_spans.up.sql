create view api_request_latency as 
  select latency_tracking.request_id as request_id, route, start_ts, total_processing_ms, processing_times, version
  from latency_tracking inner join api_request on latency_tracking.request_id = api_request.request_id
  order by start_ts desc;