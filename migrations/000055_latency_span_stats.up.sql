-- latency_span_stats: a flat row-per-span view over the nested jsonb stored
-- in latency_tracking.processing_times. The jsonb shape (a tree of spans
-- with name/elapsed/subSpans, see internal/domain/span.go) is awkward to
-- query directly; this view turns it into a relational shape so we can
-- answer questions like "what's p95 of span 'load price cache' across the
-- last 24 hours" with one SELECT.
--
-- Sample rows for a single backtest request:
--
--   span_path                                                   | depth | elapsed_ms
--   running backtest                                            |   0   | 14570
--   running backtest > setting up backtest                      |   1   | 173
--   running backtest > calculating factor scores                |   1   | 6105
--   running backtest > calculating factor scores > load price.. |   2   | 5180
--   ...
--
-- depth=0 are top-level spans; deeper rows represent nested subSpans.
-- elapsed_ms can be NULL for spans whose end() was never called (the
-- domain.Span model permits this).

CREATE VIEW latency_span_stats AS
WITH RECURSIVE walk AS (
    -- Seed: top-level spans of each profile. Skip legacy rows where
    -- processing_times is a JSON object instead of the current array
    -- shape (~100 such rows from before the Span/Profile refactor).
    SELECT
        lt.latency_tracking_id,
        lt.request_id,
        span.value AS span,
        ARRAY[span.value->>'name']::text[] AS span_path_arr,
        0 AS depth,
        CASE
            WHEN jsonb_typeof(span.value->'elapsed') = 'number'
            THEN (span.value->>'elapsed')::bigint
            ELSE NULL
        END AS elapsed_ms
    FROM latency_tracking lt,
         jsonb_array_elements(lt.processing_times) AS span(value)
    WHERE jsonb_typeof(lt.processing_times) = 'array'

    UNION ALL

    -- Recurse into each parent's subSpans (if any). The COALESCE keeps
    -- jsonb_array_elements happy when subSpans is missing entirely.
    SELECT
        walk.latency_tracking_id,
        walk.request_id,
        sub.value AS span,
        walk.span_path_arr || (sub.value->>'name'),
        walk.depth + 1,
        CASE
            WHEN jsonb_typeof(sub.value->'elapsed') = 'number'
            THEN (sub.value->>'elapsed')::bigint
            ELSE NULL
        END
    FROM walk,
         jsonb_array_elements(COALESCE(walk.span->'subSpans', '[]'::jsonb)) AS sub(value)
    WHERE jsonb_typeof(walk.span->'subSpans') = 'array'
)
SELECT
    walk.latency_tracking_id,
    walk.request_id,
    ar.route,
    ar.start_ts,
    ar.version,
    ar.duration_ms AS request_duration_ms,
    array_to_string(walk.span_path_arr, ' > ') AS span_path,
    walk.span_path_arr[array_length(walk.span_path_arr, 1)] AS span_name,
    walk.depth,
    walk.elapsed_ms
FROM walk
LEFT JOIN api_request ar ON walk.request_id = ar.request_id;
