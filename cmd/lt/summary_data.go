package main

type projectStats struct {
	Name        string `json:"name"`
	Open        int    `json:"open"`
	InProgress  int    `json:"in_progress"`
	Closed      int    `json:"closed"`
	LastUpdated string `json:"last_updated,omitempty"`
}

type topTicket struct {
	Project   string `json:"project"`
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

type summary struct {
	Projects []projectStats `json:"projects"`
	Top      []topTicket    `json:"top"`
	Totals   summaryTotals  `json:"totals"`
}

type summaryTotals struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Closed     int `json:"closed"`
	Projects   int `json:"projects"`
}

const summaryTopLimit = 5

func (s *store) summarize() (*summary, error) {
	out := &summary{
		Projects: []projectStats{},
		Top:      []topTicket{},
	}

	rows, err := s.db.Query(`
		SELECT
		  p.name,
		  COALESCE(SUM(CASE WHEN t.status='open' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN t.status='in-progress' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN t.status='closed' THEN 1 ELSE 0 END), 0),
		  COALESCE(MAX(t.updated_at), '')
		FROM projects p
		LEFT JOIN tickets t ON t.project_id = p.id
		GROUP BY p.id
		ORDER BY COALESCE(MAX(t.updated_at), '') DESC, p.name`)
	if err != nil {
		return nil, internalErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var ps projectStats
		if err := rows.Scan(&ps.Name, &ps.Open, &ps.InProgress, &ps.Closed, &ps.LastUpdated); err != nil {
			return nil, internalErr(err)
		}
		out.Projects = append(out.Projects, ps)
		out.Totals.Open += ps.Open
		out.Totals.InProgress += ps.InProgress
		out.Totals.Closed += ps.Closed
	}
	if err := rows.Err(); err != nil {
		return nil, internalErr(err)
	}
	out.Totals.Projects = len(out.Projects)

	tops, err := s.db.Query(`
		SELECT p.name, t.num, t.title, t.status, t.updated_at
		FROM tickets t
		JOIN projects p ON p.id = t.project_id
		WHERE t.status != 'closed'
		ORDER BY t.updated_at DESC, t.num DESC
		LIMIT ?`, summaryTopLimit)
	if err != nil {
		return nil, internalErr(err)
	}
	defer tops.Close()
	for tops.Next() {
		var tt topTicket
		if err := tops.Scan(&tt.Project, &tt.ID, &tt.Title, &tt.Status, &tt.UpdatedAt); err != nil {
			return nil, internalErr(err)
		}
		out.Top = append(out.Top, tt)
	}
	return out, tops.Err()
}
