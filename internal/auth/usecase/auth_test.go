package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/example/ai-restaurant-assistant-backend/internal/auth"
	repositorymodels "github.com/example/ai-restaurant-assistant-backend/internal/models/repository"
	usecasemodels "github.com/example/ai-restaurant-assistant-backend/internal/models/usecase"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/bcrypt"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"

	"github.com/google/uuid"

	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

const hashedPassword = "hashed"

func sessionID() uuid.UUID { return uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa") }
func userID() uuid.UUID    { return uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb") }

// ----- Register -----

func TestRegister_NewUser_HappyPath(t *testing.T) {
	const email = "ivan@example.com"
	const password = "secret123"
	newUserID := userID()

	users := &mockUserRepo{
		findByEmailFn: func(_ context.Context, e string) (*repositorymodels.User, error) {
			require.Equal(t, email, e)
			return nil, user.ErrNotFound
		},
		createFn: func(_ context.Context, u *repositorymodels.User) error {
			require.Equal(t, newUserID, u.ID)
			require.Equal(t, ptr(email), u.Email)
			require.Equal(t, "customer", u.Role)
			require.Equal(t, ptr(hashedPassword), u.PasswordHash)
			return nil
		},
	}
	hasher := &mockHasher{hashFn: func(p string) (string, error) {
		require.Equal(t, password, p)
		return hashedPassword, nil
	}}
	sess := &mockSessionUC{attachUserFn: func(_ context.Context, sid, uid uuid.UUID) (*usecasemodels.Session, error) {
		require.Equal(t, sessionID(), sid)
		require.Equal(t, newUserID, uid)
		return &usecasemodels.Session{ID: sid, CSRF: "rotated"}, nil
	}}
	uuidGen := &mockUUID{next: newUserID}

	uc := New(users, sess, hasher, uuidGen)
	u, s, err := uc.Register(context.Background(), sessionID(), nil, email, password)
	require.NoError(t, err)
	require.Equal(t, email, u.Email)
	require.Equal(t, "rotated", s.CSRF)
}

func TestRegister_EmailTaken(t *testing.T) {
	const email = "ivan@example.com"
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) {
			return &repositorymodels.User{Email: ptr(email), Role: "customer"}, nil
		},
	}
	uc := New(users, nil, nil, nil)

	_, _, err := uc.Register(context.Background(), sessionID(), nil, email, "secret123")
	require.ErrorIs(t, err, user.ErrEmailTaken)
}

func TestRegister_UpgradesGuestRow(t *testing.T) {
	const email = "ivan@example.com"
	guestID := userID()

	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) { return nil, user.ErrNotFound },
		findByIDFn: func(_ context.Context, id uuid.UUID) (*repositorymodels.User, error) {
			require.Equal(t, guestID, id)
			return &repositorymodels.User{ID: guestID, Role: "guest"}, nil
		},
		updateFn: func(_ context.Context, u *repositorymodels.User) error {
			require.Equal(t, "customer", u.Role, "guest повышается до customer")
			require.Equal(t, ptr(email), u.Email)
			return nil
		},
	}
	hasher := &mockHasher{hashFn: func(string) (string, error) { return hashedPassword, nil }}
	sess := &mockSessionUC{attachUserFn: func(_ context.Context, _, uid uuid.UUID) (*usecasemodels.Session, error) {
		require.Equal(t, guestID, uid)
		return &usecasemodels.Session{ID: sessionID(), CSRF: "x"}, nil
	}}
	uc := New(users, sess, hasher, &mockUUID{})

	_, _, err := uc.Register(context.Background(), sessionID(), &guestID, email, "secret123")
	require.NoError(t, err)
}

func TestRegister_AlreadyRegistered_NotGuest(t *testing.T) {
	customerID := userID()
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) { return nil, user.ErrNotFound },
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: customerID, Email: ptr("old@example.com"), Role: "customer"}, nil
		},
	}
	hasher := &mockHasher{hashFn: func(string) (string, error) { return hashedPassword, nil }}
	uc := New(users, nil, hasher, &mockUUID{})

	_, _, err := uc.Register(context.Background(), sessionID(), &customerID, "new@example.com", "secret123")
	require.ErrorIs(t, err, auth.ErrAlreadyRegistered)
}

// ----- Login -----

func TestLogin_HappyPath(t *testing.T) {
	const email = "ivan@example.com"
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) {
			return &repositorymodels.User{
				ID: userID(), Email: ptr(email),
				PasswordHash: ptr(hashedPassword), Role: "customer",
			}, nil
		},
	}
	hasher := &mockHasher{compareFn: func(h, p string) error {
		require.Equal(t, hashedPassword, h)
		require.Equal(t, "secret123", p)
		return nil
	}}
	sess := &mockSessionUC{attachUserFn: func(_ context.Context, _, uid uuid.UUID) (*usecasemodels.Session, error) {
		require.Equal(t, userID(), uid)
		return &usecasemodels.Session{CSRF: "rotated"}, nil
	}}
	uc := New(users, sess, hasher, &mockUUID{})

	u, s, err := uc.Login(context.Background(), sessionID(), email, "secret123")
	require.NoError(t, err)
	require.Equal(t, email, u.Email)
	require.Equal(t, "rotated", s.CSRF)
}

func TestLogin_UnknownEmail_InvalidCredentials(t *testing.T) {
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) { return nil, user.ErrNotFound },
	}
	uc := New(users, nil, nil, nil)
	_, _, err := uc.Login(context.Background(), sessionID(), "unknown@example.com", "anything")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials, "не раскрываем, что email не существует")
}

func TestLogin_WrongPassword_InvalidCredentials(t *testing.T) {
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) {
			return &repositorymodels.User{
				ID: userID(), Email: ptr("x"),
				PasswordHash: ptr(hashedPassword), Role: "customer",
			}, nil
		},
	}
	hasher := &mockHasher{compareFn: func(string, string) error { return bcrypt.ErrMismatch }}
	uc := New(users, nil, hasher, nil)

	_, _, err := uc.Login(context.Background(), sessionID(), "x", "wrong")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLogin_HasherInternalError_NotInvalidCredentials(t *testing.T) {
	users := &mockUserRepo{
		findByEmailFn: func(context.Context, string) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), PasswordHash: ptr("h"), Role: "customer"}, nil
		},
	}
	hasher := &mockHasher{compareFn: func(string, string) error { return errors.New("bcrypt boom") }}
	uc := New(users, nil, hasher, nil)

	_, _, err := uc.Login(context.Background(), sessionID(), "x", "y")
	require.Error(t, err)
	require.NotErrorIs(t, err, auth.ErrInvalidCredentials, "внутренняя поломка hasher не маскируется под bad credentials")
}

// ----- Logout -----

func TestLogout_DelegatesToSession(t *testing.T) {
	called := false
	sess := &mockSessionUC{destroyFn: func(_ context.Context, sid uuid.UUID) error {
		require.Equal(t, sessionID(), sid)
		called = true
		return nil
	}}
	uc := New(nil, sess, nil, nil)

	require.NoError(t, uc.Logout(context.Background(), sessionID()))
	require.True(t, called)
}

// ----- ChangePassword -----

func TestChangePassword_HappyPath(t *testing.T) {
	users := &mockUserRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), Email: ptr("x"), PasswordHash: ptr("old-hash"), Role: "customer"}, nil
		},
		updateFn: func(_ context.Context, u *repositorymodels.User) error {
			require.Equal(t, ptr("new-hash"), u.PasswordHash)
			return nil
		},
	}
	hasher := &mockHasher{
		compareFn: func(string, string) error { return nil },
		hashFn:    func(string) (string, error) { return "new-hash", nil },
	}
	uc := New(users, nil, hasher, nil)

	require.NoError(t, uc.ChangePassword(context.Background(), userID(), "old-pw", "new-pw"))
}

func TestChangePassword_GuestForbidden(t *testing.T) {
	users := &mockUserRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), Role: "guest"}, nil
		},
	}
	uc := New(users, nil, nil, nil)
	err := uc.ChangePassword(context.Background(), userID(), "x", "y")
	require.ErrorIs(t, err, user.ErrInsufficientRole)
}

func TestChangePassword_WrongCurrent_InvalidCredentials(t *testing.T) {
	users := &mockUserRepo{
		findByIDFn: func(context.Context, uuid.UUID) (*repositorymodels.User, error) {
			return &repositorymodels.User{ID: userID(), PasswordHash: ptr("h"), Role: "customer"}, nil
		},
	}
	hasher := &mockHasher{compareFn: func(string, string) error { return bcrypt.ErrMismatch }}
	uc := New(users, nil, hasher, nil)

	err := uc.ChangePassword(context.Background(), userID(), "wrong", "new-pw")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}
