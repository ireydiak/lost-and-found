package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ireydiak/shops/internal/auth"
)

// submissionPayload is the body of POST /api/submissions: a proposed shop,
// plus the target shop_id when the proposal is an update.
type submissionPayload struct {
	ShopID *int64 `json:"shop_id"`
	shopPayload
}

type submissionJSON struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	ShopID      *int64          `json:"shop_id"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	SubmittedBy string          `json:"submitted_by"`
	ReviewedBy  *string         `json:"reviewed_by"`
	CreatedAt   time.Time       `json:"created_at"`
	ReviewedAt  *time.Time      `json:"reviewed_at"`
}

// createSubmission handles POST /api/submissions (any logged-in user).
func (s *server) createSubmission(w http.ResponseWriter, r *http.Request) {
	var p submissionPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}
	kind := "create"
	if p.ShopID != nil {
		kind = "update"
	}
	if err := p.validate(kind == "create"); err != nil {
		badRequest(w, err.Error())
		return
	}
	if p.ShopID != nil {
		var exists bool
		if err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM shops WHERE shop_id = $1)`, *p.ShopID).Scan(&exists); err != nil {
			s.internalError(w, err)
			return
		}
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "shop not found"})
			return
		}
	}

	payload, err := json.Marshal(p.shopPayload)
	if err != nil {
		s.internalError(w, err)
		return
	}
	var id int64
	err = s.db.QueryRow(
		`INSERT INTO submissions (kind, shop_id, payload, submitted_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING submission_id`,
		kind, p.ShopID, payload, auth.EmailFromContext(r.Context()),
	).Scan(&id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.respondWithSubmission(w, http.StatusCreated, id)
}

// listSubmissions handles GET /api/submissions?status= (admin).
func (s *server) listSubmissions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "approved" && status != "rejected" {
		badRequest(w, "status must be pending, approved or rejected")
		return
	}
	rows, err := s.db.Query(
		`SELECT submission_id, kind, shop_id, payload, status, submitted_by, reviewed_by, created_at, reviewed_at
		 FROM submissions WHERE status = $1::submission_status ORDER BY submission_id`, status)
	if err != nil {
		s.internalError(w, err)
		return
	}
	defer rows.Close()

	out := []submissionJSON{}
	for rows.Next() {
		var sub submissionJSON
		if err := rows.Scan(&sub.ID, &sub.Kind, &sub.ShopID, &sub.Payload, &sub.Status,
			&sub.SubmittedBy, &sub.ReviewedBy, &sub.CreatedAt, &sub.ReviewedAt); err != nil {
			s.internalError(w, err)
			return
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// reviewSubmission handles POST /api/submissions/{id}/approve and /reject
// (admin). Approving executes the proposed change through the same code path
// as the direct admin routes.
func (s *server) reviewSubmission(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			badRequest(w, "invalid submission id")
			return
		}

		var (
			kind    string
			shopID  sql.NullInt64
			payload []byte
		)
		err = s.db.QueryRow(
			`SELECT kind, shop_id, payload FROM submissions
			 WHERE submission_id = $1 AND status = 'pending'`, id,
		).Scan(&kind, &shopID, &payload)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no pending submission with that id"})
			return
		}
		if err != nil {
			s.internalError(w, err)
			return
		}

		if approve {
			var p shopPayload
			if err := json.Unmarshal(payload, &p); err != nil {
				s.internalError(w, err)
				return
			}
			if kind == "create" {
				_, err = s.insertShop(&p)
			} else {
				err = s.applyShopUpdate(shopID.Int64, &p)
			}
			switch {
			case errors.Is(err, errShopNotFound):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "target shop no longer exists"})
				return
			case errors.Is(err, errIncompleteAddress) || errors.Is(err, errNoAddress):
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			case err != nil:
				s.internalError(w, err)
				return
			}
		}

		verdict := "rejected"
		if approve {
			verdict = "approved"
		}
		res, err := s.db.Exec(
			`UPDATE submissions
			 SET status = $1::submission_status, reviewed_by = $2, reviewed_at = NOW()
			 WHERE submission_id = $3 AND status = 'pending'`,
			verdict, auth.EmailFromContext(r.Context()), id)
		if err != nil {
			s.internalError(w, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "submission was reviewed concurrently"})
			return
		}
		s.respondWithSubmission(w, http.StatusOK, id)
	}
}

func (s *server) respondWithSubmission(w http.ResponseWriter, code int, id int64) {
	var sub submissionJSON
	err := s.db.QueryRow(
		`SELECT submission_id, kind, shop_id, payload, status, submitted_by, reviewed_by, created_at, reviewed_at
		 FROM submissions WHERE submission_id = $1`, id,
	).Scan(&sub.ID, &sub.Kind, &sub.ShopID, &sub.Payload, &sub.Status,
		&sub.SubmittedBy, &sub.ReviewedBy, &sub.CreatedAt, &sub.ReviewedAt)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, code, sub)
}
