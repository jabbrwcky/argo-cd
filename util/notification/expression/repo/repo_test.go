package repo

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/util/notification/argocd/mocks"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"
)

const (
	gitRepo   = "https://github.com/example/config.git"
	otherRepo = "https://github.com/example/other.git"
	helmRepo  = "https://charts.example.com"
	sha       = "abc123"
)

func newApp(spec map[string]any) *unstructured.Unstructured {
	spec["project"] = "default"
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "app", "namespace": "argocd"},
		"spec":       spec,
	}}
}

func TestGetCommitMetadata(t *testing.T) {
	t.Parallel()

	meta := &shared.CommitMetadata{Message: "fix: something", Author: "dev"}

	t.Run("single source", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewService(t)
		svc.EXPECT().GetCommitMetadata(mock.Anything, gitRepo, sha, "default").Return(meta, nil).Once()

		got, err := getCommitMetadata(sha, newApp(map[string]any{
			"source": map[string]any{"repoURL": gitRepo, "path": "."},
		}), svc)
		require.NoError(t, err)
		assert.Equal(t, meta, got)
	})

	t.Run("multiple sources skip helm and use the git source", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewService(t)
		svc.EXPECT().GetCommitMetadata(mock.Anything, gitRepo, sha, "default").Return(meta, nil).Once()

		got, err := getCommitMetadata(sha, newApp(map[string]any{
			"sources": []any{
				map[string]any{"repoURL": helmRepo, "chart": "app", "targetRevision": "1.0.0"},
				map[string]any{"repoURL": gitRepo, "ref": "values"},
			},
		}), svc)
		require.NoError(t, err)
		assert.Equal(t, meta, got)
	})

	t.Run("multiple git sources try each until the commit is found", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewService(t)
		svc.EXPECT().GetCommitMetadata(mock.Anything, otherRepo, sha, "default").Return(nil, errors.New("not found")).Once()
		svc.EXPECT().GetCommitMetadata(mock.Anything, gitRepo, sha, "default").Return(meta, nil).Once()

		got, err := getCommitMetadata(sha, newApp(map[string]any{
			"sources": []any{
				map[string]any{"repoURL": otherRepo, "path": "."},
				map[string]any{"repoURL": gitRepo, "path": "."},
			},
		}), svc)
		require.NoError(t, err)
		assert.Equal(t, meta, got)
	})

	t.Run("commit in no source returns the last error", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewService(t)
		svc.EXPECT().GetCommitMetadata(mock.Anything, gitRepo, sha, "default").Return(nil, errors.New("not found")).Once()

		_, err := getCommitMetadata(sha, newApp(map[string]any{
			"sources": []any{
				map[string]any{"repoURL": helmRepo, "chart": "app", "targetRevision": "1.0.0"},
				map[string]any{"repoURL": gitRepo, "ref": "values"},
			},
		}), svc)
		require.EqualError(t, err, "not found")
	})

	t.Run("no git source panics", func(t *testing.T) {
		t.Parallel()
		svc := mocks.NewService(t)

		assert.PanicsWithError(t, "failed to get application source repo URL", func() {
			_, _ = getCommitMetadata(sha, newApp(map[string]any{
				"sources": []any{
					map[string]any{"repoURL": helmRepo, "chart": "app", "targetRevision": "1.0.0"},
				},
			}), svc)
		})
	})
}
