package storage

import (
	"context"

	pq "github.com/lib/pq"
)

// FindDiscordByFaceitIDs: devuelve mapa faceit_user_id -> discord_user_id
func (r *UserRepo) FindDiscordByFaceitIDs(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT faceit_user_id, discord_user_id
  FROM user_links
 WHERE faceit_user_id = ANY($1) AND deleted_at IS NULL
`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var fid, did string
		if err := rows.Scan(&fid, &did); err != nil {
			return nil, err
		}
		out[fid] = did
	}
	return out, rows.Err()
}
