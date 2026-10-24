package main

import (
	"fmt"
	"time"

	emailgrpc "github.com/miebyte/emailsender/api/grpc"
	"github.com/miebyte/emailsender/internal/config"
	"github.com/miebyte/emailsender/internal/email"
	"github.com/miebyte/emailsender/internal/queue"
	"github.com/miebyte/emailsender/internal/smtp"
	"github.com/miebyte/emailsender/internal/storage"
	"github.com/miebyte/emailsender/internal/storage/models"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
	"google.golang.org/grpc"
)

var (
	port       = flags.Int("port", 28081, "internal HTTP health check port")
	configFlag = flags.Struct("config", &config.Config{}, "email sender configs")
)

func init() {
	share.SetServiceName("emailsender")
}

func main() {
	logging.PanicError(run())
}

func run() error {
	flags.Parse(flags.WithRemoteConfig())
	defer flags.Close()

	loader, err := configFlag()
	logging.PanicError(err)

	conf := loader.Load()

	db, err := conf.MySQL.DialMysqlGorm()
	logging.PanicError(err)
	logging.PanicError(db.AutoMigrate(models.AllModels()...))

	redisClient, err := conf.Redis.DialGORedisClient()
	logging.PanicError(err)
	defer redisClient.Close()

	sender, err := smtp.NewSender(*conf.SMTP)
	logging.PanicError(err)

	tasks, err := queue.NewRedisQueue(redisClient, conf.QueueName, time.Duration(conf.Redis.Timeout)*time.Second)
	logging.PanicError(err)

	records, err := storage.NewRecordStore(db)
	logging.PanicError(err)

	service, err := email.NewService(sender, tasks, records)
	logging.PanicError(err)

	grpcServer := grpc.NewServer()
	emailgrpc.SetupGrpcServer(grpcServer, service)
	srv := cores.NewCores(
		cores.WithInternalHTTPAddress(fmt.Sprintf(":%d", port())),
		cores.WithServiceAddress(conf.GrpcAddr()),
		cores.WithGrpcServer(grpcServer),
		cores.WithConsulRegister(),
		cores.WithWorker(service.Run, time.Duration(conf.Redis.Timeout)*time.Second+10*time.Second),
	)
	return cores.Start(srv)
}
