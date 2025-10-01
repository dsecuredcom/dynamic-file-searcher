package domain_test

import (
	"testing"

	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/domain"
)

func collectParts(host string, cfg *config.Config) []string {
	parts := make([]string, 0)
	domain.StreamDomainParts(host, cfg, func(word string) {
		parts = append(parts, word)
	})
	return parts
}

func TestStreamDomainPartsLimitRespected(t *testing.T) {
	host := "admin-api-portal.stage.internal.example.com"

	baseCfg := &config.Config{
		NoEnvAppending:           true,
		EnvRemoving:              false,
		AppendByPassesToWords:    false,
		IgnoreBasePathSlash:      false,
		MaxGeneratedWordsPerHost: 0,
	}

	unlimited := collectParts(host, baseCfg)
	if len(unlimited) < 5 {
		t.Fatalf("expected unlimited generation to produce at least 5 parts, got %d", len(unlimited))
	}

	limit := 3
	limitedCfg := *baseCfg
	limitedCfg.MaxGeneratedWordsPerHost = limit

	limited := collectParts(host, &limitedCfg)
	if len(limited) != limit {
		t.Fatalf("expected limit %d to yield %d parts, got %d", limit, limit, len(limited))
	}
}

func TestStreamDomainPartsLimitAppliesToEnvVariants(t *testing.T) {
	host := "portal.dev.example.com"

	unlimitedCfg := &config.Config{
		AppendEnvList: []string{"dev", "prod"},
		EnvRemoving:   true,
	}

	unlimited := collectParts(host, unlimitedCfg)
	if len(unlimited) < 4 {
		t.Fatalf("expected unlimited generation to produce env variants, got %d", len(unlimited))
	}

	limitedCfg := *unlimitedCfg
	limitedCfg.MaxGeneratedWordsPerHost = 2

	limited := collectParts(host, &limitedCfg)
	if len(limited) != limitedCfg.MaxGeneratedWordsPerHost {
		t.Fatalf("expected limit to restrict env variants to %d parts, got %d", limitedCfg.MaxGeneratedWordsPerHost, len(limited))
	}
}
