package content

import (
	"context"
	"log"

	"github.com/indieinfra/scribble/server/util"
)

type NoopContentStore struct{}

func (cs *NoopContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	log.Println("Received no-op create request - dumping request information:")
	log.Printf("Type: %v", doc.Type)
	log.Printf("Properties:")
	for key, value := range doc.Properties {
		log.Printf("\t%v: %v", key, value)
	}
	return "https://noop.example.org/noop", true, nil
}

func (cs *NoopContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	log.Println("Received no-op update request - dumping request information:")
	log.Printf("Url: %v", url)
	log.Println("Replacements:")
	for key, value := range replacements {
		log.Printf("\t%v: %v", key, value)
	}
	log.Println("Additions:")
	for key, value := range additions {
		log.Printf("\t%v: %v", key, value)
	}
	log.Println("Deletions:")
	switch v := deletions.(type) {
	case []any:
		for _, value := range v {
			log.Printf("\t- %v", value)
		}
	case map[string][]any:
		for key, value := range v {
			log.Printf("\t%v: %v", key, value)
		}
	}
	return url, nil
}

func (cs *NoopContentStore) Delete(ctx context.Context, url string) error {
	log.Println("Received no-op delete request - dumping request information:")
	log.Printf("Url: %v", url)
	return nil
}

func (cs *NoopContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	log.Println("Received no-op undelete request - dumping request information:")
	log.Printf("Url: %v", url)
	return url, false, nil
}

func (cs *NoopContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	log.Println("Received no-op get-content request - dumping request information and generating bogus response")
	log.Printf("Url: %v", url)
	return &util.Mf2Document{
		Type: []string{"h-entry"},
		Properties: map[string][]any{
			"name":    {"This is a bogus title"},
			"content": {"This is bogus content, sentence one", "sentence two!"},
			"url":     {url},
		},
	}, nil
}

func (cs *NoopContentStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	log.Println("Received no-op exists-by-slug request - dumping request information:")
	log.Printf("Slug: %v", slug)
	return false, nil
}
