//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../mocks/core_repository.go . CoreRepository
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../mocks/github_gateway.go . GitHubGateway

package usecase

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prchecklist "github.com/motemen/prchecklist/v2"
	"github.com/motemen/prchecklist/v2/lib/repository_mock"
)

func TestUseCase_GetChecklist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repository_mock.NewMockCoreRepository(ctrl)
	github := NewMockGitHubGateway(ctrl)

	clRef := prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 1, Stage: "default"}

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		clRef,
		true,
	).Return(&prchecklist.PullRequest{
		Owner: "test",
		Repo:  "test",
		Commits: []prchecklist.Commit{
			{Message: "Merge pull request #2 "},
		},
		ConfigBlobID: "DUMMY-CONFIG-BLOB-ID",
	}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 2},
		false,
	).Return(&prchecklist.PullRequest{}, context.Background(), nil)

	github.EXPECT().GetBlob(
		gomock.Any(),
		clRef,
		"DUMMY-CONFIG-BLOB-ID",
	).Return(
		[]byte(`---
stages:
  - qa
  - production
`),
		nil,
	)

	repo.EXPECT().GetChecks(gomock.Any(), clRef).
		Return(prchecklist.Checks{}, nil)

	repo.EXPECT().GetUsers(gomock.Any(), gomock.Len(0)).
		Return(map[int]prchecklist.GitHubUser{}, nil)

	app := New(github, repo)
	ctx := context.Background()

	cl, err := app.GetChecklist(ctx, clRef)

	assert.NoError(t, err)
	assert.Equal(
		t, cl.Config.Stages, []string{"qa", "production"},
	)
}

func TestUseCase_GetChecklist_SkipsItemsByLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repository_mock.NewMockCoreRepository(ctrl)
	github := NewMockGitHubGateway(ctrl)

	clRef := prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 1, Stage: "qa"}

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		clRef,
		true,
	).Return(&prchecklist.PullRequest{
		Owner: "test",
		Repo:  "test",
		Commits: []prchecklist.Commit{
			{Message: "Merge pull request #2 "},
			{Message: "Merge pull request #3 "},
			{Message: "Merge pull request #4 "},
		},
		ConfigBlobID: "DUMMY-CONFIG-BLOB-ID",
	}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 2},
		false,
	).Return(&prchecklist.PullRequest{Number: 2, Labels: []string{"no-qa"}}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 3},
		false,
	).Return(&prchecklist.PullRequest{Number: 3, Labels: []string{"no-production-check"}}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 4},
		false,
	).Return(&prchecklist.PullRequest{Number: 4}, context.Background(), nil)

	github.EXPECT().GetBlob(
		gomock.Any(),
		clRef,
		"DUMMY-CONFIG-BLOB-ID",
	).Return(
		[]byte(`---
stages:
  - qa
  - production
skip:
  labels:
    - name: no-qa
    - name: no-production-check
      stages:
        - production
`),
		nil,
	)

	repo.EXPECT().GetChecks(gomock.Any(), clRef).
		Return(prchecklist.Checks{}, nil)

	repo.EXPECT().GetUsers(gomock.Any(), gomock.Len(0)).
		Return(map[int]prchecklist.GitHubUser{}, nil)

	app := New(github, repo)

	cl, err := app.GetChecklist(context.Background(), clRef)

	assert.NoError(t, err)
	assert.True(t, cl.Item(2).Skipped, "#2 has a label to skip on all stages")
	assert.False(t, cl.Item(3).Skipped, "#3 has a label to skip only on production")
	assert.False(t, cl.Item(4).Skipped, "#4 has no labels")
}

func TestUseCase_GetChecklist_ResponsibleUserOfBotPullRequests(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repository_mock.NewMockCoreRepository(ctrl)
	github := NewMockGitHubGateway(ctrl)

	clRef := prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 1, Stage: "qa"}

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		clRef,
		true,
	).Return(&prchecklist.PullRequest{
		Owner: "test",
		Repo:  "test",
		Commits: []prchecklist.Commit{
			{Message: "Merge pull request #2 "},
			{Message: "Merge pull request #3 "},
			{Message: "Merge pull request #4 "},
			{Message: "Merge pull request #5 "},
		},
		ConfigBlobID: "DUMMY-CONFIG-BLOB-ID",
	}, context.Background(), nil)

	renovate := prchecklist.GitHubUserSimple{Login: "renovate"}
	merger := prchecklist.GitHubUserSimple{Login: "merger"}
	assignee := prchecklist.GitHubUserSimple{Login: "assignee"}

	// The gateway may return a cached pull request; it must not be modified.
	botPullReq := &prchecklist.PullRequest{Number: 2, User: renovate, Author: renovate, MergedBy: merger}

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 2},
		false,
	).Return(botPullReq, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 3},
		false,
	).Return(&prchecklist.PullRequest{Number: 3, User: assignee, Author: renovate, Assignee: assignee, MergedBy: merger}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 4},
		false,
	).Return(&prchecklist.PullRequest{Number: 4, User: prchecklist.GitHubUserSimple{Login: "author"}, Author: prchecklist.GitHubUserSimple{Login: "author"}, MergedBy: merger}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 5},
		false,
	).Return(&prchecklist.PullRequest{Number: 5, User: renovate, Author: renovate, MergedBy: renovate}, context.Background(), nil)

	github.EXPECT().GetBlob(
		gomock.Any(),
		clRef,
		"DUMMY-CONFIG-BLOB-ID",
	).Return(
		[]byte(`---
stages:
  - qa
bots:
  - renovate[bot]
`),
		nil,
	)

	repo.EXPECT().GetChecks(gomock.Any(), clRef).
		Return(prchecklist.Checks{}, nil)

	repo.EXPECT().GetUsers(gomock.Any(), gomock.Len(0)).
		Return(map[int]prchecklist.GitHubUser{}, nil)

	app := New(github, repo)

	cl, err := app.GetChecklist(context.Background(), clRef)

	assert.NoError(t, err)
	assert.Equal(t, "merger", cl.Item(2).User.Login, "#2 is a bot pull request without assignees")
	assert.True(t, cl.Item(2).ByBot)
	assert.Equal(t, "renovate", botPullReq.User.Login, "the pull request returned by the gateway is not modified")
	assert.False(t, cl.Item(2).Unassigned)
	assert.Equal(t, "assignee", cl.Item(3).User.Login, "#3 is a bot pull request with an assignee")
	assert.True(t, cl.Item(3).ByBot)
	assert.False(t, cl.Item(3).Unassigned)
	assert.Equal(t, "author", cl.Item(4).User.Login, "#4 is not a bot pull request")
	assert.False(t, cl.Item(4).ByBot)
	assert.False(t, cl.Item(4).Unassigned)
	assert.True(t, cl.Item(5).Unassigned, "#5 is a bot pull request merged by itself")
	assert.True(t, cl.Item(5).ByBot)
	assert.Equal(t, "renovate", cl.Item(5).User.Login)
}

func TestAddCheckEvents_UnassignedItem(t *testing.T) {
	renovate := prchecklist.GitHubUserSimple{Login: "renovate"}
	checklist := &prchecklist.Checklist{
		PullRequest: &prchecklist.PullRequest{},
		Items: []*prchecklist.ChecklistItem{
			{
				PullRequest: &prchecklist.PullRequest{Number: 2, User: renovate, Author: renovate, MergedBy: renovate},
				CheckedBy:   []prchecklist.GitHubUser{{Login: "motemen"}},
				ByBot:       true,
				Unassigned:  true,
			},
			{
				PullRequest: &prchecklist.PullRequest{Number: 3, User: prchecklist.GitHubUserSimple{Login: "author"}},
				CheckedBy:   []prchecklist.GitHubUser{},
			},
		},
	}

	events := addCheckEvents(checklist, 2, prchecklist.GitHubUser{Login: "motemen"})

	require.Len(t, events, 1, "no completeChecksOfUserEvent for the bot")
	assert.IsType(t, addCheckEvent{}, events[0])

	checklist.Items[0].Unassigned = false
	events = addCheckEvents(checklist, 2, prchecklist.GitHubUser{Login: "motemen"})

	require.Len(t, events, 2)
	assert.IsType(t, completeChecksOfUserEvent{}, events[1])
}

func setupMocks(clRef prchecklist.ChecklistRef, github *MockGitHubGateway, repo *repository_mock.MockCoreRepository) {
	github.EXPECT().GetPullRequest(
		gomock.Any(),
		clRef,
		true,
	).Return(&prchecklist.PullRequest{
		Owner: "test",
		Repo:  "test",
		Commits: []prchecklist.Commit{
			{Message: "Merge pull request #2 "},
			{Message: "Merge pull request #3 "},
		},
	}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 2},
		false,
	).Return(&prchecklist.PullRequest{
		Number: 2,
	}, context.Background(), nil)

	github.EXPECT().GetPullRequest(
		gomock.Any(),
		prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 3},
		false,
	).Return(&prchecklist.PullRequest{
		Number: 3,
	}, context.Background(), nil)

	repo.EXPECT().GetChecks(gomock.Any(), clRef).
		Return(prchecklist.Checks{}, nil)

	repo.EXPECT().GetUsers(gomock.Any(), gomock.Len(0)).
		Return(map[int]prchecklist.GitHubUser{}, nil)
}

func TestUsecase_AddCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clRef := prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 1, Stage: "default"}
	repo := repository_mock.NewMockCoreRepository(ctrl)
	github := NewMockGitHubGateway(ctrl)

	setupMocks(clRef, github, repo)

	repo.EXPECT().AddCheck(
		gomock.Any(),
		clRef,
		"2",
		gomock.Any(),
	)

	app := New(github, repo)
	ctx := context.Background()

	cl, err := app.AddCheck(
		ctx,
		clRef,
		2,
		prchecklist.GitHubUser{
			ID:    1,
			Login: "test",
		},
	)

	assert.NoError(t, err)
	t.Log(cl)
}

func TestUsecase_RemoveCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clRef := prchecklist.ChecklistRef{Owner: "test", Repo: "test", Number: 1, Stage: "default"}
	repo := repository_mock.NewMockCoreRepository(ctrl)
	github := NewMockGitHubGateway(ctrl)

	setupMocks(clRef, github, repo)

	repo.EXPECT().RemoveCheck(
		gomock.Any(),
		clRef,
		"2",
		gomock.Any(),
	)

	app := New(github, repo)
	ctx := context.Background()

	cl, err := app.RemoveCheck(
		ctx,
		clRef,
		2,
		prchecklist.GitHubUser{
			ID:    1,
			Login: "test",
		},
	)

	assert.NoError(t, err)
	t.Log(cl)
}
