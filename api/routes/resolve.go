package routes

import (
	"root/database"

	"github.com/go-redis/redis"
	"github.com/gofiber/fiber/v2"
)

func ResolveURL(ctx *fiber.Ctx) error {
	url := ctx.Params("url")
	r := database.CreateClient(0)
	defer r.Close()

	value, err := r.Get(url).Result()
	if err == redis.Nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "short not found"})
	} else if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "cannot connect to DB"})
	}

	rInr := database.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr("counter")

	return ctx.Redirect(value, 301)
}
