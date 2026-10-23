package main

import (
	"flag"

	"github.com/miebyte/emailsender/internal/storage/models"
	"gorm.io/gen"
)

//go:generate go run . -out ../../internal/storage/query

func main() {
	out := flag.String("out", "internal/storage/query", "generated query directory")
	flag.Parse()

	g := gen.NewGenerator(gen.Config{
		OutPath:       *out,
		WithUnitTest:  false,
		FieldNullable: true,
		Mode:          gen.WithQueryInterface,
	})

	g.ApplyBasic(models.AllModels()...)
	g.Execute()
}
