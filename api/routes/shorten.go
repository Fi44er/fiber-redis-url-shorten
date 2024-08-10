package routes

import (
	"os"
	"root/database"
	"root/helpers"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"custom_short"`
	Expiry      time.Duration `json:"expiry"`
}
type response struct {
	URL             string        `json:"url"`
	CustomShort     string        `json:"custom_short"`
	Expiry          time.Duration `json:"expiry"`
	XRateRemaining  int           `json:"x_rate_remaining"`
	XRateLimitReset time.Duration `json:"x_rate_limit_rest"`
}

func ShortenURL(ctx *fiber.Ctx) error {
	body := new(request)

	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse JSON"})
	}

	// implement rate limiting

	r2 := database.CreateClient(1)
	defer r2.Close()
	value, err := r2.Get(ctx.IP()).Result()
	if err == redis.Nil {
		_ = r2.Set(ctx.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
	} else {
		_, _ = r2.Get(ctx.IP()).Result()
		valInt, _ := strconv.Atoi(value)
		if valInt <= 0 {
			limit, _ := r2.TTL(ctx.IP()).Result()
			return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":           "Rate limit exceeded",
				"rate_limit_rest": limit / time.Nanosecond,
			})
		}
	}

	// check if the input if an actual URl

	if !govalidator.IsURL(body.URL) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad URL"})
	}

	// check for domain error

	if !helpers.RemoveDomainError(body.URL) {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": `You can't hack the system ¯\_(ツ)_/¯`})
	}

	// ecforce https, SSL

	body.URL = helpers.EnforceHTTP(body.URL)

	var id string

	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	r := database.CreateClient(0)
	defer r.Close()

	value, _ = r.Get(id).Result()
	if value != "" {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL custom short alredy in used"})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = r.Set(id, body.URL, body.Expiry*9600*time.Second).Err()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to connect to server"})
	}

	resp := response{
		URL:             body.URL,
		CustomShort:     "",
		Expiry:          body.Expiry,
		XRateRemaining:  10,
		XRateLimitReset: 30,
	}

	r2.Decr(ctx.IP())

	value, _ = r2.Get(ctx.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(value)

	ttl, _ := r2.TTL(ctx.IP()).Result()
	resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id

	return ctx.Status(fiber.StatusOK).JSON(resp)
}
