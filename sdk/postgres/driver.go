package postgres

import (
	"fmt"
	"log"
	"time"

	"code.finan.one/finan-one-be/fo-utils/l"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var ll = l.New()

type Config struct {
	Username string `json:"username" mapstructure:"username"`
	Password string `json:"password" mapstructure:"password"`
	Host     string `json:"host" mapstructure:"host"`
	Port     string `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
	SSLMode  string `json:"sslmode" mapstructure:"sslmode"` // thêm SSLMode cho Postgres
}

type DB struct {
	DB *gorm.DB
}

func (s *DB) Close() {
	sqlDB, err := s.DB.DB()
	if err == nil {
		sqlDB.Close()
	}
}

func MustConnectDB(cfg Config) *DB {
	ll.Info("Start connect to postgres")
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Database, cfg.Password,
	)
	ll.Debug("connection string", l.String("conn", dsn))

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: dsn,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Sử dụng tên bảng số ít
		},
	})

	if err != nil {
		log.Fatalf("error when init postgres db connection: %s", err)
	}

	sqlDB, _ := db.DB()

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDB.SetMaxIdleConns(50)

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(100)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ll.Info("connected to postgres")
	return &DB{DB: db}
}
