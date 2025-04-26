package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mattdavis90/immich-stacker/client"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Stack struct {
	IDs    []openapi_types.UUID
	Parent *openapi_types.UUID
}

func (s Stack) Stackable() bool {
	return s.Parent != nil && len(s.IDs) > 0
}

type Stats struct {
	Stackable    int
	NotStackable int
	Success      int
	Failed       int
}

type Config struct {
	APIKey         string `env:"API_KEY"`
	Endpoint       string `env:"ENDPOINT"`
	Match          string `env:"MATCH"`
	Parent         string `env:"PARENT"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"INFO"`
	DebugHTTP      bool   `env:"DEBUG_HTTP" envDefault:"false"`
	CompareCreated bool   `env:"COMPARE_CREATED" envDefault:"false"`
	InsecureTLS    bool   `env:"INSECURE_TLS" envDefault:"false"`
	ReadOnly       bool   `env:"READ_ONLY" envDefault:"false"`
}

type HTTPLogger struct{}

const VERSION = "v1.5.0"

func (hl HTTPLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody := ""
	respBody := ""

	if req.Body != nil {
		buf, e := io.ReadAll(req.Body)
		if e != nil {
			log.Fatal().Err(e).Msg("Failed to read HTTP request")
		} else {
			reqRdr := io.NopCloser(bytes.NewBuffer(buf))
			req.Body = reqRdr
			reqBody = string(buf)
		}
	}

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.Body != nil {
		buf, e := io.ReadAll(resp.Body)
		if e != nil {
			log.Fatal().Err(e).Msg("Failed to read HTTP response")
		} else {
			respRdr := io.NopCloser(bytes.NewBuffer(buf))
			resp.Body = respRdr
			respBody = string(buf)
		}
	}

	log.Debug().
		Str("method", req.Method).
		Str("path", req.URL.Path).
		Any("req_hdr", req.Header).
		Str("req_body", reqBody).
		Any("resp_hdr", resp.Header).
		Str("resp_body", respBody).
		Int("status", resp.StatusCode).
		Msg("Request")

	return resp, err
}

func main() {
	godotenv.Load()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cfg := Config{}
	opts := env.Options{RequiredIfNoDef: true, Prefix: "IMMICH_"}
	if err := env.ParseWithOptions(&cfg, opts); err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	zerolog.SetGlobalLevel(level)

	m, err := regexp.Compile(cfg.Match)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	p, err := regexp.Compile(cfg.Parent)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	sp, err := securityprovider.NewSecurityProviderApiKey("header", "x-api-key", cfg.APIKey)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	log.Info().Str("version", VERSION).Msg("Starting immish-stacker")
	log.Info().Str("endpoint", cfg.Endpoint).Msg("Connecting to Immich")

	if cfg.InsecureTLS {
		log.Warn().Msg("Insecure TLS connections enabled")
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	var hc http.Client
	if cfg.DebugHTTP {
		hl := HTTPLogger{}
		hc = http.Client{Transport: hl}
	} else {
		hc = http.Client{}
	}

	c, err := client.NewClientWithResponses(
		cfg.Endpoint,
		client.WithHTTPClient(&hc),
		client.WithRequestEditorFn(sp.Intercept),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	ctx := context.Background()

	verResp, err := c.GetServerVersionWithResponse(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if verResp.StatusCode() != http.StatusOK {
		log.Fatal().Int("status", verResp.StatusCode()).Msg("Expected HTTP 200")
	}
	if verResp.JSON200 == nil {
		log.Fatal().Msg("nil return")
	}
	v := verResp.JSON200
	log.Info().Int("major", v.Major).Int("minor", v.Minor).Int("patch", v.Patch).Msg("Server version")

	log.Info().Msg("Requesting all time buckets")

	total := 0
	stacks := make(map[string]*Stack)
	t := true
	resp, err := c.GetTimeBucketsWithResponse(ctx, &client.GetTimeBucketsParams{Size: client.MONTH, WithStacked: &t})
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if resp.StatusCode() != http.StatusOK {
		log.Fatal().Int("status", resp.StatusCode()).Msg("Expected HTTP 200")
	}
	if resp.JSON200 == nil {
		log.Fatal().Msg("nil return")
	}
	for _, tb := range *resp.JSON200 {
		log.Debug().Str("time_bucket", tb.TimeBucket).Int("count", tb.Count).Msg("Requesting time bucket")

		resp, err := c.GetTimeBucketWithResponse(ctx, &client.GetTimeBucketParams{TimeBucket: tb.TimeBucket, Size: client.MONTH, WithStacked: &t})
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		if resp.StatusCode() != http.StatusOK {
			log.Fatal().Int("status", resp.StatusCode()).Msg("Expected HTTP 200")
		}
		if resp.JSON200 == nil {
			log.Fatal().Msg("nil return")
		}

		total += len(*resp.JSON200)

		log.Debug().Str("time_bucket", tb.TimeBucket).Int("expected", tb.Count).Int("got", len(*resp.JSON200)).Msg("Retrieved time bucket")
		for _, a := range *resp.JSON200 {
			if a.Stack != nil && a.Stack.AssetCount > 0 {
				continue
			}

			if m.Match([]byte(a.OriginalFileName)) {
				id := openapi_types.UUID(uuid.MustParse(a.Id))
				key := string(m.ReplaceAll([]byte(a.OriginalFileName), []byte("")))
				if cfg.CompareCreated {
					key += "_" + a.FileCreatedAt.Local().String()
				}

				s, ok := stacks[key]
				if !ok {
					s = &Stack{
						IDs: make([]uuid.UUID, 0),
					}
					stacks[key] = s
				}

				if p.Match([]byte(a.OriginalFileName)) {
					s.Parent = &id
				} else {
					s.IDs = append(s.IDs, id)
				}
			}
		}
	}

	log.Info().Int("total", total).Int("matches", len(stacks)).Msg("Retrieved assets")

	stats := Stats{}

	for f, s := range stacks {
		if s.Stackable() {
			stats.Stackable++

			log.Debug().Str("filename", f).Msg("Stacking")

			// Generate a slice of UUIDs with the parent first
			assetIDs := []openapi_types.UUID{*s.Parent}
			for _, a := range s.IDs {
				if !slices.Contains(assetIDs, a) {
					assetIDs = append(assetIDs, a)
				}
			}

			if !cfg.ReadOnly {
				resp, err := c.CreateStackWithResponse(ctx, client.StackCreateDto{
					AssetIds: assetIDs,
				})
				if err != nil {
					log.Error().Err(err)
					stats.Failed++
				} else if resp.StatusCode() != http.StatusCreated {
					log.Error().Int("status", resp.StatusCode()).Msg("Expected HTTP 201")
					stats.Failed++
				} else {
					log.Info().Str("filename", f).Msg("Created stack")
					stats.Success++
				}
			}
		} else {
			log.Debug().Str("filename", f).Msg("Skipped")
			stats.NotStackable++
		}
	}

	log.Info().
		Int("stackable", stats.Stackable).
		Int("success", stats.Success).
		Int("failed", stats.Failed).
		Int("not_stackable", stats.NotStackable).
		Msg("Finished")
}
