package metrics

import (
	"context"
	"errors"

	skillsvc "github.com/Mininglamp-OSS/octo-marketplace/internal/service/skill"
)

// SkillService is the subset of the skill service needed for visibility checks.
type SkillService interface {
	Get(ctx context.Context, id, spaceID, userID string) (*skillsvc.SkillItem, error)
}

// SkillResolver checks whether a skill exists and is visible to the caller.
type SkillResolver struct {
	skillSvc SkillService
}

// NewSkillResolver creates a SkillResolver.
func NewSkillResolver(skillSvc SkillService) *SkillResolver {
	return &SkillResolver{skillSvc: skillSvc}
}

// CanView returns true if the skill exists and is visible to the caller.
// Returns (false, nil) for not-found/not-visible; propagates internal errors.
func (r *SkillResolver) CanView(ctx context.Context, resourceID string, caller Caller) (bool, error) {
	item, err := r.skillSvc.Get(ctx, resourceID, caller.SpaceID, caller.UID)
	if err != nil {
		if errors.Is(err, skillsvc.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return item != nil, nil
}
