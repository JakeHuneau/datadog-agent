// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build trivy && (docker || containerd || crio)

package trivy

import (
	"context"
	"errors"
	"os"
	"testing"

	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
	"github.com/DataDog/datadog-agent/pkg/sbom"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPopulateImageExportsOnce checks that the image is exported once, and
// that a failed export returns its error again rather than exporting anew.
func TestPopulateImageExportsOnce(t *testing.T) {
	errExport := errors.New("export failed")
	tests := []struct {
		name    string
		opener  opener
		wantErr error
	}{
		{
			name:    "export fails",
			opener:  func() (v1.Image, error) { return nil, errExport },
			wantErr: errExport,
		},
		{
			name:   "export succeeds",
			opener: func() (v1.Image, error) { return empty.Image, nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			img := &image{opener: func() (v1.Image, error) {
				calls++
				return tt.opener()
			}}
			for range 2 {
				err := img.populateImage()
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NoError(t, err)
				}
			}
			assert.Equal(t, 1, calls)
		})
	}
}

// TestCheckExportSpace checks that an export needs 20% more than the image
// size.
func TestCheckExportSpace(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, checkExportSpace(dir, 1))
	assert.ErrorContains(t, checkExportSpace(dir, 1<<60), "not enough disk space")
}

// TestImageOpenerChecksSpace checks that the export of an image too large for
// the disk fails before it saves the image, when the disk usage is checked.
func TestImageOpenerChecksSpace(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "export")
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	errSave := errors.New("image save")
	saves := 0
	save := func(context.Context, []string, ...client.ImageSaveOption) (client.ImageSaveResult, error) {
		saves++
		return nil, errSave
	}
	huge := &workloadmeta.ContainerImageMetadata{SizeBytes: 1 << 60}

	_, err = imageOpener(t.Context(), "docker", "alpine:3.20", f, save, huge, sbom.ScanOptions{CheckDiskUsage: true})()
	assert.ErrorContains(t, err, "not enough disk space")
	assert.Equal(t, 0, saves)

	_, err = imageOpener(t.Context(), "docker", "alpine:3.20", f, save, huge, sbom.ScanOptions{})()
	assert.ErrorIs(t, err, errSave)
	assert.Equal(t, 1, saves)
}
