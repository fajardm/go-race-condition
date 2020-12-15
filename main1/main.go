package main

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Voucher struct {
	ID    string `db:"id"`
	Quota int    `db:"quota"`
}

type Repository struct {
	db *sqlx.DB
}

func (r *Repository) Get(id string) (*Voucher, error) {
	var voucher Voucher
	err := r.db.QueryRowx(`SELECT id, quota FROM vouchers WHERE id=$1`, id).StructScan(&voucher)
	return &voucher, err
}

func (r *Repository) Update(id string, quota int) error {
	_, err := r.db.Exec("UPDATE vouchers SET quota=$1 WHERE id=$2", quota, id)
	return err
}

func main() {
	db, err := sqlx.Connect("postgres", "user=root password=secret dbname=gobackend sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	repo := Repository{db: db}

	app := fiber.New()

	app.Get("/vouchers/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")

		existing, err := repo.Get(id)
		fmt.Println("existing", existing.Quota)
		if err != nil {
			return err
		}

		quotaUpdated := existing.Quota - 1

		if err := repo.Update(id, quotaUpdated); err != nil {
			return err
		}

		return c.JSON(quotaUpdated)
	})

	log.Fatal(app.Listen(":3001"))
}
