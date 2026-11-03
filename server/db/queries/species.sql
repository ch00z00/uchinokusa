-- name: UpsertSpecies :one
-- slug が同じ行があれば上書きする。返す inserted は、新しく追加した行なら true になる
-- （PostgreSQL では、追加したばかりの行の xmax は 0 になる。上書きした行は 0 ではない）
INSERT INTO species (
  slug, name_ja, scientific_name, aliases, light_level, min_temp_c, humidity_level,
  airflow_need, water_tendency, outdoor_ok, recommended_care, attributes, search_text
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (slug) DO UPDATE SET
  name_ja          = EXCLUDED.name_ja,
  scientific_name  = EXCLUDED.scientific_name,
  aliases          = EXCLUDED.aliases,
  light_level      = EXCLUDED.light_level,
  min_temp_c       = EXCLUDED.min_temp_c,
  humidity_level   = EXCLUDED.humidity_level,
  airflow_need     = EXCLUDED.airflow_need,
  water_tendency   = EXCLUDED.water_tendency,
  outdoor_ok       = EXCLUDED.outdoor_ok,
  recommended_care = EXCLUDED.recommended_care,
  attributes       = EXCLUDED.attributes,
  search_text      = EXCLUDED.search_text
RETURNING (xmax = 0) AS inserted;

-- name: SearchSpecies :many
-- pattern は '%検索語%'（検索語の % と _ はエスケープ済み）。query は正規化した検索語そのもの
-- 一致した位置が前（和名）のものほど先に、同じなら和名の短いものから並べる
SELECT * FROM species
WHERE search_text LIKE sqlc.arg(pattern)::text
ORDER BY strpos(search_text, sqlc.arg(query)::text), char_length(name_ja), name_ja
LIMIT sqlc.arg(max_results)::integer;
