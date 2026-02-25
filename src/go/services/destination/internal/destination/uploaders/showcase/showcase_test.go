package showcase

import (
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	"github.com/stretchr/testify/assert"
)

func TestShowcaseUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "showcase", u.Name())
}

func TestCalculateExpiration(t *testing.T) {
	now := time.Now()

	// Hobbyist test
	hobbyRec := &user.Record{
		UserProfile: &pbuser.UserProfile{
			UserId: "hobbyist",
		},
	}
	expHobby := calculateExpiration(hobbyRec, now)
	assert.NotNil(t, expHobby)
	assert.True(t, expHobby.After(now.AddDate(0, 0, 29)))

	// Athlete test
	athleteRec := &user.Record{
		UserProfile: &pbuser.UserProfile{
			UserId: "athlete",
			Tier:   pbuser.UserTier_USER_TIER_ATHLETE,
		},
	}
	expAthlete := calculateExpiration(athleteRec, now)
	assert.Nil(t, expAthlete)
}
