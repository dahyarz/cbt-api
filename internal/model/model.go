package model

import (
	"time"
	"gorm.io/datatypes"
)

type User struct {
	ID                    string         `gorm:"column:id;primaryKey;type:uuid"`
	Username              string         `gorm:"column:username"`
	PasswordHash          string         `gorm:"column:password_hash"`
	Name                  string         `gorm:"column:name"`
	Role                  string         `gorm:"column:role"`
	Status                string         `gorm:"column:status"`
	RefreshToken          *string        `gorm:"column:refresh_token"`
	LastLogin             *time.Time     `gorm:"column:last_login"`
	LastLogout            *time.Time     `gorm:"column:last_logout"`
	LastIP                *string        `gorm:"column:last_ip"`
	LoginCount            int            `gorm:"column:login_count"`
	FailedLoginAttempts   int            `gorm:"column:failed_login_attempts"`
	LastFailedLogin       *time.Time     `gorm:"column:last_failed_login"`
	LockedUntil           *time.Time     `gorm:"column:locked_until"`
	ActiveSessionID       *string        `gorm:"column:active_session_id"`
	SessionExpiresAt      *time.Time     `gorm:"column:session_expires_at"`
	SessionDeviceInfo     datatypes.JSON `gorm:"column:session_device_info"`
	SessionIPAddress      *string        `gorm:"column:session_ip_address"`
	SessionCreatedAt      *time.Time     `gorm:"column:session_created_at"`
	AllowMultipleSessions bool           `gorm:"column:allow_multiple_sessions"`
	CreatedAt             time.Time      `gorm:"column:created_at"`
	UpdatedAt             time.Time      `gorm:"column:updated_at"`
	DeletedAt             *time.Time     `gorm:"column:deleted_at"`
}

func (User) TableName() string { return "cbt.users" }

type Exam struct {
	ID              string     `gorm:"column:id;primaryKey;type:uuid"`
	Title           string     `gorm:"column:title"`
	Description     *string    `gorm:"column:description"`
	DurationMinutes int        `gorm:"column:duration_minutes"`
	RandomQuestion  bool       `gorm:"column:random_question"`
	ShuffleOptions  bool       `gorm:"column:shuffle_options"`
	ShowResult      bool       `gorm:"column:show_result"`
	CategoriesID    string     `gorm:"column:categories_id"`
	StartTest       *time.Time `gorm:"column:start_test"`
	EndTest         *time.Time `gorm:"column:end_test"`
	Token           *string    `gorm:"column:token"`
	MaxViolations   int        `gorm:"column:max_violations"`
	ReviewMode      string     `gorm:"column:review_mode"`
	QuestionLimit   *int       `gorm:"column:question_limit"`
	ShowToken       bool       `gorm:"column:show_token"`
	TokenTTLMinutes int        `gorm:"column:token_ttl_minutes"`
	TokenExpiresAt  *time.Time `gorm:"column:token_expires_at"`
	IsKurikulum     bool       `gorm:"column:is_kurikulum"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (Exam) TableName() string { return "cbt.exams" }

type Question struct {
	ID               string         `gorm:"column:id;primaryKey;type:uuid"`
	CategoryID       *string        `gorm:"column:category_id"`
	Type             string         `gorm:"column:type"`
	Text             string         `gorm:"column:text"`
	Options          datatypes.JSON `gorm:"column:options"`
	CorrectAnswer    datatypes.JSON `gorm:"column:correct_answer"`
	Points           int            `gorm:"column:points"`
	OrderIndex       int            `gorm:"column:order_index"`
	IsRequired       bool           `gorm:"column:is_required"`
	ShuffleFillBlank bool           `gorm:"column:shuffle_fill_blank"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at"`
}

func (Question) TableName() string { return "cbt.questions" }

type ExamAttempt struct {
	ID                     string         `gorm:"column:id;primaryKey;type:uuid"`
	UserID                 string         `gorm:"column:user_id"`
	ExamID                 string         `gorm:"column:exam_id"`
	RandomizationSeed      *int64         `gorm:"column:randomization_seed"`
	ShuffledQuestionsOrder datatypes.JSON `gorm:"column:shuffled_questions_order"`
	StartedAt              *time.Time     `gorm:"column:started_at"`
	FinishedAt             *time.Time     `gorm:"column:finished_at"`
	FinalScore             *int           `gorm:"column:final_score"`
	MaxScore               *int           `gorm:"column:max_score"`
	Answers                datatypes.JSON `gorm:"column:answers"`
	Status                 string         `gorm:"column:status"`
	IPAddress              *string        `gorm:"column:ip_address"`
	UserAgent              *string        `gorm:"column:user_agent"`
	QuestionScores         datatypes.JSON `gorm:"column:question_scores"`
	AttemptNumber          int            `gorm:"column:attempt_number"`
	IsActive               bool           `gorm:"column:is_active"`
	SubmitReason           *string        `gorm:"column:submit_reason"`
	SubmittedBy            *string        `gorm:"column:submitted_by"`
	ArchivedAt             *time.Time     `gorm:"column:archived_at"`
	CreatedAt              time.Time      `gorm:"column:created_at"`
	UpdatedAt              time.Time      `gorm:"column:updated_at"`
	DeletedAt              *time.Time     `gorm:"column:deleted_at"`
}

func (ExamAttempt) TableName() string { return "cbt.exam_attempts" }

type ExamQuestionSet struct {
	ID            string    `gorm:"column:id;primaryKey;type:uuid"`
	AttemptID     string    `gorm:"column:attempt_id"`
	QuestionID    string    `gorm:"column:question_id"`
	ShuffledIndex int       `gorm:"column:shuffled_index"`
	OriginalOrder int       `gorm:"column:original_order"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (ExamQuestionSet) TableName() string { return "cbt.exam_question_sets" }

type ExamSession struct {
	ID            string         `gorm:"column:id;primaryKey;type:uuid"`
	AttemptID     string         `gorm:"column:attempt_id"`
	RedisKey      string         `gorm:"column:redis_key"`
	LastHeartbeat *time.Time     `gorm:"column:last_heartbeat"`
	SessionData   datatypes.JSON `gorm:"column:session_data"`
	ExpiredAt     *time.Time     `gorm:"column:expired_at"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
}

func (ExamSession) TableName() string { return "cbt.exam_sessions" }

type AnswerTemp struct {
	ID         string         `gorm:"column:id;primaryKey;type:uuid"`
	AttemptID  string         `gorm:"column:attempt_id"`
	QuestionID string         `gorm:"column:question_id"`
	Answer     datatypes.JSON `gorm:"column:answer"`
	Media      datatypes.JSON `gorm:"column:media"`
	IsFlagged  bool           `gorm:"column:is_flagged"`
	AnsweredAt *time.Time     `gorm:"column:answered_at"`
	UpdatedAt  *time.Time     `gorm:"column:updated_at"`
	CreatedAt  *time.Time     `gorm:"column:created_at"`
}

func (AnswerTemp) TableName() string { return "cbt.answers_temp" }

type DeviceFingerprint struct {
	ID              string     `gorm:"column:id;primaryKey;type:uuid"`
	UserID          string     `gorm:"column:user_id"`
	FingerprintHash string     `gorm:"column:fingerprint_hash"`
	DeviceName      *string    `gorm:"column:device_name"`
	IPAddress       *string    `gorm:"column:ip_address"`
	UserAgent       *string    `gorm:"column:user_agent"`
	IsTrusted       bool       `gorm:"column:is_trusted"`
	IsBlocked       bool       `gorm:"column:is_blocked"`
	FirstSeen       time.Time  `gorm:"column:first_seen"`
	LastSeen        time.Time  `gorm:"column:last_seen"`
	TimesSeen       int        `gorm:"column:times_seen"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (DeviceFingerprint) TableName() string { return "cbt.device_fingerprints" }

type ExamBlockedUser struct {
	ID        string     `gorm:"column:id;primaryKey;type:uuid"`
	ExamID    string     `gorm:"column:exam_id"`
	UserID    string     `gorm:"column:user_id"`
	Reason    *string    `gorm:"column:reason"`
	BlockedAt time.Time  `gorm:"column:blocked_at"`
	UnblockedAt *time.Time `gorm:"column:unblocked_at"`
}

func (ExamBlockedUser) TableName() string { return "cbt.exam_blocked_users" }

type LoginAttempt struct {
	ID              string    `gorm:"column:id;primaryKey;type:uuid"`
	UserID          *string   `gorm:"column:user_id"`
	Identifier      string    `gorm:"column:identifier"`
	IPAddress       string    `gorm:"column:ip_address"`
	UserAgent       *string   `gorm:"column:user_agent"`
	Success         bool      `gorm:"column:success"`
	FingerprintHash *string   `gorm:"column:fingerprint_hash"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (LoginAttempt) TableName() string { return "cbt.login_attempts" }

type LoginSessionAudit struct {
	ID              string         `gorm:"column:id;primaryKey;type:uuid"`
	UserID          string         `gorm:"column:user_id"`
	Event           string         `gorm:"column:event"`
	OldSessionID    *string        `gorm:"column:old_session_id"`
	NewSessionID    *string        `gorm:"column:new_session_id"`
	IPAddress       *string        `gorm:"column:ip_address"`
	UserAgent       *string        `gorm:"column:user_agent"`
	Details         datatypes.JSON `gorm:"column:details"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
}

func (LoginSessionAudit) TableName() string { return "cbt.login_session_audits" }
