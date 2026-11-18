package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/sasadu/cbt-api/internal/grading"
	"github.com/sasadu/cbt-api/internal/model"
)

type questionDTO struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Text      string         `json:"text"`
	Options   datatypes.JSON `json:"options"`
	Points    int            `json:"points"`
	Order     int            `json:"order"`
	Shuffle   bool           `json:"shuffle_fill_blank"`
}

func (d *Deps) attemptResponse(c *fiber.Ctx, a *model.ExamAttempt, e *model.Exam) error {
	var sets []model.ExamQuestionSet
	d.DB.Where("attempt_id = ?", a.ID).Order("shuffled_index ASC").Find(&sets)

	ids := make([]string, 0, len(sets))
	for _, s := range sets { ids = append(ids, s.QuestionID) }

	var qs []model.Question
	if len(ids) > 0 {
		d.DB.Where("id IN ?", ids).Find(&qs)
	}
	qmap := map[string]model.Question{}
	for _, q := range qs { qmap[q.ID] = q }

	out := make([]questionDTO, 0, len(sets))
	for _, s := range sets {
		q, ok := qmap[s.QuestionID]
		if !ok { continue }
		out = append(out, questionDTO{
			ID: q.ID, Type: q.Type, Text: q.Text, Options: q.Options,
			Points: q.Points, Order: s.ShuffledIndex, Shuffle: q.ShuffleFillBlank,
		})
	}

	answers := map[string]json.RawMessage{}
	if vals, err := d.Redis.HGetAll(context.Background(), "answers:"+a.ID).Result(); err == nil && len(vals) > 0 {
		for k, v := range vals { answers[k] = json.RawMessage(v) }
	} else {
		var rows []model.AnswerTemp
		d.DB.Where("attempt_id = ?", a.ID).Find(&rows)
		for _, r := range rows { answers[r.QuestionID] = json.RawMessage(r.Answer) }
	}

	return c.JSON(fiber.Map{
		"attempt_id":       a.ID,
		"exam_id":          a.ExamID,
		"status":           a.Status,
		"started_at":       a.StartedAt,
		"remaining_seconds": d.remainingSeconds(a, e.DurationMinutes),
		"server_time":      time.Now(),
		"questions":        out,
		"answers":          answers,
	})
}

func (d *Deps) GetAttempt(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}
	return d.attemptResponse(c, a, &e)
}

func (d *Deps) fail(c *fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) { return errJSON(c, fe.Code, fe.Message) }
	return errJSON(c, 500, "terjadi kesalahan")
}

func (d *Deps) saveOne(a *model.ExamAttempt, e *model.Exam, qid string, answer json.RawMessage, flagged *bool) error {
	if d.remainingSeconds(a, e.DurationMinutes) <= 0 {
		return fiber.NewError(fiber.StatusForbidden, "waktu ujian sudah habis")
	}
	var cnt int64
	d.DB.Model(&model.ExamQuestionSet{}).
		Where("attempt_id = ? AND question_id = ?", a.ID, qid).Count(&cnt)
	if cnt == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "soal bukan bagian dari attempt ini")
	}

	ctx := context.Background()
	if err := d.Redis.HSet(ctx, "answers:"+a.ID, qid, string(answer)).Err(); err != nil {
		return err
	}

	now := time.Now()
	var row model.AnswerTemp
	err := d.DB.Where("attempt_id = ? AND question_id = ?", a.ID, qid).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.AnswerTemp{
			ID: uuid.NewString(), AttemptID: a.ID, QuestionID: qid,
			Answer: datatypes.JSON(answer), AnsweredAt: &now, UpdatedAt: &now, CreatedAt: &now,
		}
		if flagged != nil { row.IsFlagged = *flagged }
		return d.DB.Create(&row).Error
	}
	if err != nil { return err }

	upd := map[string]any{"answer": datatypes.JSON(answer), "updated_at": now}
	if flagged != nil { upd["is_flagged"] = *flagged }
	return d.DB.Model(&model.AnswerTemp{}).Where("id = ?", row.ID).Updates(upd).Error
}

func (d *Deps) SaveAnswer(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }
	if a.Status != "in_progress" { return errJSON(c, 409, "ujian sudah selesai") }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil { return errJSON(c, 404, "ujian tidak ditemukan") }

	var req struct {
		Answer    json.RawMessage `json:"answer"`
		IsFlagged *bool           `json:"is_flagged"`
	}
	if err := c.BodyParser(&req); err != nil { return errJSON(c, 400, "body tidak valid") }

	if err := d.saveOne(a, &e, c.Params("question_id"), req.Answer, req.IsFlagged); err != nil {
		return d.fail(c, err)
	}
	return c.JSON(fiber.Map{"status": "saved", "question_id": c.Params("question_id"), "saved_at": time.Now()})
}

func (d *Deps) SaveAnswersBatch(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }
	if a.Status != "in_progress" { return errJSON(c, 409, "ujian sudah selesai") }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil { return errJSON(c, 404, "ujian tidak ditemukan") }

	var req struct {
		Answers []struct {
			QuestionID string          `json:"question_id"`
			Answer     json.RawMessage `json:"answer"`
			IsFlagged  *bool           `json:"is_flagged"`
		} `json:"answers"`
	}
	if err := c.BodyParser(&req); err != nil { return errJSON(c, 400, "body tidak valid") }

	saved := 0
	for _, item := range req.Answers {
		if err := d.saveOne(a, &e, item.QuestionID, item.Answer, item.IsFlagged); err != nil {
			return d.fail(c, err)
		}
		saved++
	}
	return c.JSON(fiber.Map{"status": "saved", "count": saved, "saved_at": time.Now()})
}

func (d *Deps) FlagQuestion(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	var req struct {
		QuestionID string `json:"question_id"`
		Flagged    bool   `json:"flagged"`
	}
	if err := c.BodyParser(&req); err != nil || req.QuestionID == "" {
		return errJSON(c, 400, "question_id wajib")
	}

	ctx := context.Background()
	key := "flags:" + a.ID
	if req.Flagged {
		d.Redis.SAdd(ctx, key, req.QuestionID)
	} else {
		d.Redis.SRem(ctx, key, req.QuestionID)
	}
	d.DB.Model(&model.AnswerTemp{}).
		Where("attempt_id = ? AND question_id = ?", a.ID, req.QuestionID).
		Update("is_flagged", req.Flagged)

	return c.JSON(fiber.Map{"status": "ok", "question_id": req.QuestionID, "flagged": req.Flagged})
}

func (d *Deps) Heartbeat(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil { return errJSON(c, 404, "ujian tidak ditemukan") }

	now := time.Now()
	d.DB.Model(&model.ExamSession{}).Where("attempt_id = ?", a.ID).Updates(map[string]any{
		"last_heartbeat": now,
	})

	left := d.remainingSeconds(a, e.DurationMinutes)
	return c.JSON(fiber.Map{
		"status": "ok", "remaining_seconds": left, "server_time": now,
		"submit_required": left == 0,
	})
}

func (d *Deps) SubmitAttempt(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	if a.Status != "in_progress" {
		return c.JSON(fiber.Map{"status": a.Status, "final_score": a.FinalScore, "max_score": a.MaxScore})
	}

	ctx := context.Background()
	lockKey := "lock:submit:" + a.ID
	ok, _ := d.Redis.SetNX(ctx, lockKey, "1", 15*time.Second).Result()
	if !ok { return errJSON(c, 409, "submit sedang diproses") }
	defer d.Redis.Del(ctx, lockKey)

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil { return errJSON(c, 404, "ujian tidak ditemukan") }

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&req)
	reason := req.Reason
	if reason == "" { reason = "manual" }

	var sets []model.ExamQuestionSet
	d.DB.Where("attempt_id = ?", a.ID).Find(&sets)
	ids := make([]string, 0, len(sets))
	for _, s := range sets { ids = append(ids, s.QuestionID) }

	var questions []model.Question
	if len(ids) > 0 { d.DB.Where("id IN ?", ids).Find(&questions) }

	answers := map[string]json.RawMessage{}
	if vals, err := d.Redis.HGetAll(ctx, "answers:"+a.ID).Result(); err == nil && len(vals) > 0 {
		for k, v := range vals { answers[k] = json.RawMessage(v) }
	} else {
		var rows []model.AnswerTemp
		d.DB.Where("attempt_id = ?", a.ID).Find(&rows)
		for _, r := range rows { answers[r.QuestionID] = json.RawMessage(r.Answer) }
	}

	final, max, scores, _ := grading.Grade(questions, answers)
	scoresJSON, _ := json.Marshal(scores)
	answersJSON, _ := json.Marshal(answers)
	now := time.Now()

	upd := map[string]any{
		"status": "submitted", "finished_at": now, "final_score": final, "max_score": max,
		"question_scores": datatypes.JSON(scoresJSON), "answers": datatypes.JSON(answersJSON),
		"submit_reason": reason, "submitted_by": "student", "updated_at": now,
	}
	if err := d.DB.Model(&model.ExamAttempt{}).Where("id = ?", a.ID).Updates(upd).Error; err != nil {
		return errJSON(c, 500, "gagal submit")
	}

	d.Redis.Del(ctx, "answers:"+a.ID, "flags:"+a.ID, "attempt:"+a.ID+":deadline")
	d.DB.Model(&model.ExamSession{}).Where("attempt_id = ?", a.ID).Update("expired_at", now)

	return c.JSON(fiber.Map{
		"status": "submitted", "final_score": final, "max_score": max,
		"submitted_at": now, "submit_reason": reason,
	})
}

func (d *Deps) GetResult(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}
	if !e.ShowResult {
		return errJSON(c, 403, "hasil ujian tidak ditampilkan")
	}
	return c.JSON(fiber.Map{
		"attempt_id": a.ID, "status": a.Status, "final_score": a.FinalScore,
		"max_score": a.MaxScore, "finished_at": a.FinishedAt, "submit_reason": a.SubmitReason,
	})
}

func (d *Deps) GetReview(c *fiber.Ctx) error {
	a, err := d.loadAttempt(c)
	if err != nil { return d.fail(c, err) }

	var e model.Exam
	if err := d.DB.Where("id = ?", a.ExamID).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}
	if e.ReviewMode == "none" {
		return errJSON(c, 403, "review tidak diizinkan")
	}
	if a.Status == "in_progress" {
		return errJSON(c, 409, "ujian belum selesai")
	}

	var sets []model.ExamQuestionSet
	d.DB.Where("attempt_id = ?", a.ID).Order("shuffled_index ASC").Find(&sets)
	ids := make([]string, 0, len(sets))
	for _, s := range sets { ids = append(ids, s.QuestionID) }

	var questions []model.Question
	if len(ids) > 0 { d.DB.Where("id IN ?", ids).Find(&questions) }
	qmap := map[string]model.Question{}
	for _, q := range questions { qmap[q.ID] = q }

	answers := map[string]json.RawMessage{}
	var rows []model.AnswerTemp
	d.DB.Where("attempt_id = ?", a.ID).Find(&rows)
	for _, r := range rows { answers[r.QuestionID] = json.RawMessage(r.Answer) }

	out := make([]fiber.Map, 0, len(sets))
	for _, s := range sets {
		q, ok := qmap[s.QuestionID]
		if !ok { continue }
		item := fiber.Map{
			"question_id": q.ID, "type": q.Type, "text": q.Text,
			"options": q.Options, "points": q.Points,
			"your_answer": answers[q.ID],
		}
		if e.ReviewMode == "with_key" {
			item["correct_answer"] = q.CorrectAnswer
		}
		out = append(out, item)
	}

	return c.JSON(fiber.Map{
		"attempt_id": a.ID, "review_mode": e.ReviewMode,
		"final_score": a.FinalScore, "max_score": a.MaxScore, "items": out,
	})
}
