package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/sasadu/cbt-api/internal/model"
)

func (d *Deps) ListExams(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	now := time.Now()

	var exams []model.Exam
	if err := d.DB.Where("deleted_at IS NULL").
		Where("(start_test IS NULL OR start_test <= ?)", now).
		Where("(end_test IS NULL OR end_test >= ?)", now).
		Order("start_test ASC NULLS LAST").Find(&exams).Error; err != nil {
		return errJSON(c, 500, "gagal mengambil daftar ujian")
	}

	var blocked []string
	d.DB.Model(&model.ExamBlockedUser{}).
		Where("user_id = ? AND unblocked_at IS NULL", uid).Pluck("exam_id", &blocked)
	blockedSet := map[string]bool{}
	for _, b := range blocked { blockedSet[b] = true }

	out := make([]fiber.Map, 0, len(exams))
	for _, e := range exams {
		if blockedSet[e.ID] { continue }
		item := fiber.Map{
			"id": e.ID, "title": e.Title, "duration_minutes": e.DurationMinutes,
			"start_test": e.StartTest, "end_test": e.EndTest,
			"show_token": e.ShowToken, "show_result": e.ShowResult,
		}
		out = append(out, item)
	}
	return c.JSON(fiber.Map{"data": out, "server_time": now})
}

func (d *Deps) GetExam(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	var e model.Exam
	if err := d.DB.Where("id = ? AND deleted_at IS NULL", c.Params("id")).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}

	var active int64
	d.DB.Model(&model.ExamAttempt{}).
		Where("user_id = ? AND exam_id = ? AND is_active = true AND status = 'in_progress'", uid, e.ID).
		Count(&active)

	item := fiber.Map{
		"id": e.ID, "title": e.Title, "description": e.Description,
		"duration_minutes": e.DurationMinutes, "start_test": e.StartTest, "end_test": e.EndTest,
		"show_token": e.ShowToken, "show_result": e.ShowResult, "review_mode": e.ReviewMode,
		"has_active_attempt": active > 0,
	}
	if e.ShowToken && e.Token != nil {
		item["token"] = *e.Token
		item["token_expires_at"] = e.TokenExpiresAt
	}
	return c.JSON(item)
}

func (d *Deps) ValidateToken(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return errJSON(c, 400, "token wajib")
	}
	var e model.Exam
	if err := d.DB.Where("id = ? AND deleted_at IS NULL", c.Params("id")).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}
	if !d.checkToken(context.Background(), &e, req.Token) {
		return errJSON(c, 401, "token tidak valid atau kedaluwarsa")
	}
	return c.JSON(fiber.Map{"valid": true, "token_expires_at": e.TokenExpiresAt})
}

func (d *Deps) checkToken(ctx context.Context, e *model.Exam, token string) bool {
	if token == "" { return false }
	if stored, err := d.Redis.Get(ctx, "exam:token:"+e.ID).Result(); err == nil {
		if subtle.ConstantTimeCompare([]byte(stored), []byte(token)) != 1 { return false }
		return e.TokenExpiresAt == nil || time.Now().Before(*e.TokenExpiresAt)
	}
	if e.Token == nil || subtle.ConstantTimeCompare([]byte(*e.Token), []byte(token)) != 1 { return false }
	return e.TokenExpiresAt == nil || time.Now().Before(*e.TokenExpiresAt)
}

func (d *Deps) StartExam(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	examID := c.Params("id")
	ctx := context.Background()

	var req struct {
		Token string `json:"token"`
	}
	_ = c.BodyParser(&req)

	var e model.Exam
	if err := d.DB.Where("id = ? AND deleted_at IS NULL", examID).First(&e).Error; err != nil {
		return errJSON(c, 404, "ujian tidak ditemukan")
	}

	now := time.Now()
	if e.StartTest != nil && now.Before(*e.StartTest) { return errJSON(c, 403, "ujian belum dimulai") }
	if e.EndTest != nil && now.After(*e.EndTest) { return errJSON(c, 403, "ujian sudah berakhir") }

	var blocked int64
	d.DB.Model(&model.ExamBlockedUser{}).
		Where("exam_id = ? AND user_id = ? AND unblocked_at IS NULL", examID, uid).Count(&blocked)
	if blocked > 0 { return errJSON(c, 403, "Anda diblokir dari ujian ini") }

	if !d.checkToken(ctx, &e, req.Token) {
		return errJSON(c, 401, "token tidak valid atau kedaluwarsa")
	}

	lockKey := "lock:attempt:" + uid + ":" + examID
	ok, _ := d.Redis.SetNX(ctx, lockKey, "1", 10*time.Second).Result()
	if !ok { return errJSON(c, 409, "sedang memproses, coba lagi") }
	defer d.Redis.Del(ctx, lockKey)

	var active model.ExamAttempt
	if err := d.DB.Where("user_id = ? AND exam_id = ? AND is_active = true AND status = 'in_progress' AND deleted_at IS NULL", uid, examID).
		First(&active).Error; err == nil {
		return d.attemptResponse(c, &active, &e)
	}

	var questions []model.Question
	if err := d.DB.Where("category_id = ? AND deleted_at IS NULL", e.CategoriesID).
		Order("order_index ASC, id ASC").Find(&questions).Error; err != nil {
		return errJSON(c, 500, "gagal mengambil soal")
	}
	if len(questions) == 0 { return errJSON(c, 422, "ujian belum memiliki soal") }

	seed := now.UnixNano()
	r := rand.New(rand.NewSource(seed))
	if e.RandomQuestion {
		r.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
	}
	if e.QuestionLimit != nil && *e.QuestionLimit > 0 && *e.QuestionLimit < len(questions) {
		questions = questions[:*e.QuestionLimit]
	}

	orderIDs := make([]string, len(questions))
	for i, q := range questions { orderIDs[i] = q.ID }
	orderJSON, _ := json.Marshal(orderIDs)

	ip := c.IP()
	ua := c.Get("User-Agent")
	attempt := model.ExamAttempt{
		ID: uuid.NewString(), UserID: uid, ExamID: examID,
		RandomizationSeed: &seed, ShuffledQuestionsOrder: datatypes.JSON(orderJSON),
		StartedAt: &now, Status: "in_progress", IPAddress: &ip, UserAgent: &ua,
		AttemptNumber: 1, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	tx := d.DB.Begin()
	if err := tx.Create(&attempt).Error; err != nil { tx.Rollback(); return errJSON(c, 500, "gagal membuat attempt") }
	for i, q := range questions {
		if err := tx.Create(&model.ExamQuestionSet{
			ID: uuid.NewString(), AttemptID: attempt.ID, QuestionID: q.ID,
			ShuffledIndex: i, OriginalOrder: q.OrderIndex, CreatedAt: now,
		}).Error; err != nil { tx.Rollback(); return errJSON(c, 500, "gagal menyimpan urutan soal") }
	}
	deadline := now.Add(time.Duration(e.DurationMinutes) * time.Minute)
	if err := tx.Create(&model.ExamSession{
		ID: uuid.NewString(), AttemptID: attempt.ID, RedisKey: "exam:attempt:" + attempt.ID,
		LastHeartbeat: &now, ExpiredAt: &deadline, CreatedAt: now,
	}).Error; err != nil { tx.Rollback(); return errJSON(c, 500, "gagal membuat sesi") }
	tx.Commit()

	d.Redis.Set(ctx, "attempt:"+attempt.ID+":deadline", deadline.Unix(), time.Until(deadline))
	return d.attemptResponse(c, &attempt, &e)
}
