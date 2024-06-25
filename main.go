package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/deepmap/oapi-codegen/v2/pkg/securityprovider"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mattdavis90/immich-stacker/client"
	openapi_types "github.com/oapi-codegen/runtime/types"
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

func getEnv(e string) string {
	ret := os.Getenv(e)
	if ret == "" {
		log.Fatalf("%s env is required", e)
	}
	return ret
}

func main() {
	godotenv.Load()

	api_key := getEnv("IMMICH_API_KEY")
	endpoint := getEnv("IMMICH_ENDPOINT")
	match := getEnv("IMMICH_MATCH")
	parent := getEnv("IMMICH_PARENT")

	m, err := regexp.Compile(match)
	if err != nil {
		log.Fatal(err)
	}

	p, err := regexp.Compile(parent)
	if err != nil {
		log.Fatal(err)
	}

	sp, err := securityprovider.NewSecurityProviderApiKey("header", "x-api-key", api_key)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connecting to Immich on %s\n", endpoint)

	hc := http.Client{}

	c, err := client.NewClientWithResponses(
		endpoint,
		client.WithHTTPClient(&hc),
		client.WithRequestEditorFn(sp.Intercept),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	log.Println("Requesting all assets")

    assets := make([]client.AssetResponseDto, 0)
    var page float32 = 1

    for {
        t := true
        var s float32 = 1000
        resp, err := c.SearchMetadataWithResponse(ctx, client.MetadataSearchDto{WithStacked: &t, Size: &s, Page: &page})
        if err != nil {
            log.Fatal(err)
        }
        if resp.StatusCode() != http.StatusOK {
            log.Fatalf("Expected HTTP 200 but received %d", resp.StatusCode())
        }
        if resp.JSON200 == nil {
            log.Fatal("nil return")
        }
        assets = append(assets, resp.JSON200.Assets.Items...)
        if resp.JSON200.Assets.NextPage == nil {
            break
        }
        page += 1
    }

	log.Printf("Retrieved %d assets", len(assets))

	stacks := make(map[string]*Stack)

	for _, a := range assets {
		if a.StackCount != nil && *a.StackCount > 0 {
			continue
		}

		if m.Match([]byte(a.OriginalFileName)) {
			id := openapi_types.UUID(uuid.MustParse(a.Id))
			key := string(m.ReplaceAll([]byte(a.OriginalFileName), []byte("")))

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

	log.Printf("Found %d rule matches", len(stacks))

	stats := Stats{}

	for f, s := range stacks {
		if s.Stackable() {
			stats.Stackable++

			resp, err := c.UpdateAssetsWithResponse(ctx, client.AssetBulkUpdateDto{
				Ids:           s.IDs,
				StackParentId: s.Parent,
			})
			if err != nil {
				log.Println(err)
				stats.Failed++
			} else if resp.StatusCode() != http.StatusNoContent {
				log.Printf("Expected HTTP 204 but received %d\n", resp.StatusCode())
				stats.Failed++
			} else {
				log.Printf("Created stack for %s\n", f)
				stats.Success++
			}
		} else {
			stats.NotStackable++
		}
	}

	log.Printf(
		"Finished: %d Stackable, %d Succeeded, %d Failed, %d Not Stackable\n",
		stats.Stackable,
		stats.Success,
		stats.Failed,
		stats.NotStackable,
	)
}
