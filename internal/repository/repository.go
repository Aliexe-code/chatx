package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"websocket-demo/internal/db"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(queries *db.Queries) *Repository {
	return &Repository{
		queries: queries,
	}
}

// User operations
func (r *Repository) CreateUser(ctx context.Context, username, email, passwordHash string) (db.User, error) {
	return r.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
}

func (r *Repository) GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error) {
	return r.queries.GetUserByID(ctx, id)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	return r.queries.GetUserByUsername(ctx, username)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return r.queries.GetUserByEmail(ctx, email)
}

// Room operations
func (r *Repository) CreateRoom(ctx context.Context, name string, private pgtype.Bool, passwordHash pgtype.Text, creatorID pgtype.UUID) (db.Room, error) {
	return r.queries.CreateRoom(ctx, db.CreateRoomParams{
		Name:         name,
		Private:      private,
		PasswordHash: passwordHash,
		CreatorID:    creatorID,
	})
}

func (r *Repository) GetRoomByID(ctx context.Context, id pgtype.UUID) (db.Room, error) {
	return r.queries.GetRoomByID(ctx, id)
}

func (r *Repository) GetRoomByName(ctx context.Context, name string) (db.Room, error) {
	return r.queries.GetRoomByName(ctx, name)
}

func (r *Repository) ListRooms(ctx context.Context, limit, offset int32) ([]db.Room, error) {
	return r.queries.ListRooms(ctx, db.ListRoomsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *Repository) GetAllRooms(ctx context.Context) ([]db.Room, error) {
	return r.queries.ListRooms(ctx, db.ListRoomsParams{
		Limit:  1000, // Large limit to get all rooms
		Offset: 0,
	})
}

func (r *Repository) DeleteRoom(ctx context.Context, id pgtype.UUID) error {
	return r.queries.DeleteRoom(ctx, id)
}

// Message operations
func (r *Repository) CreateMessage(ctx context.Context, roomID, userID pgtype.UUID, content string) (db.Message, error) {
	return r.queries.CreateMessage(ctx, db.CreateMessageParams{
		RoomID:  roomID,
		UserID:  userID,
		Content: content,
	})
}

func (r *Repository) ListMessagesByRoom(ctx context.Context, roomID pgtype.UUID, limit, offset int32) ([]db.ListMessagesByRoomRow, error) {
	return r.queries.ListMessagesByRoom(ctx, db.ListMessagesByRoomParams{
		RoomID: roomID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *Repository) ListRecentMessagesByRoom(ctx context.Context, roomID pgtype.UUID, limit int32) ([]db.ListRecentMessagesByRoomRow, error) {
	return r.queries.ListRecentMessagesByRoom(ctx, db.ListRecentMessagesByRoomParams{
		RoomID: roomID,
		Limit:  limit,
	})
}

// Room member operations
func (r *Repository) AddRoomMember(ctx context.Context, roomID, userID pgtype.UUID) error {
	_, err := r.queries.AddRoomMember(ctx, db.AddRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	return err
}

func (r *Repository) RemoveRoomMember(ctx context.Context, roomID, userID pgtype.UUID) error {
	return r.queries.RemoveRoomMember(ctx, db.RemoveRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
}

func (r *Repository) GetRoomMembers(ctx context.Context, roomID pgtype.UUID) ([]db.GetRoomMembersRow, error) {
	return r.queries.GetRoomMembers(ctx, roomID)
}

func (r *Repository) IsRoomMember(ctx context.Context, roomID, userID pgtype.UUID) (bool, error) {
	return r.queries.IsRoomMember(ctx, db.IsRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
}

func (r *Repository) GetRoomMemberCount(ctx context.Context, roomID pgtype.UUID) (int64, error) {
	count, err := r.queries.GetRoomMemberCount(ctx, roomID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetQueries returns the underlying queries object
func (r *Repository) GetQueries() *db.Queries {
	return r.queries
}

// User profile management
func (r *Repository) UpdateUserUsername(ctx context.Context, id pgtype.UUID, username string) (db.User, error) {
	return r.queries.UpdateUserUsername(ctx, db.UpdateUserUsernameParams{
		ID:       id,
		Username: username,
	})
}

func (r *Repository) UpdateUserPassword(ctx context.Context, id pgtype.UUID, passwordHash string) (db.User, error) {
	return r.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           id,
		PasswordHash: passwordHash,
	})
}

func (r *Repository) UpdateUserLastLogin(ctx context.Context, id pgtype.UUID, lastLogin pgtype.Timestamptz) (db.User, error) {
	return r.queries.UpdateUserLastLogin(ctx, db.UpdateUserLastLoginParams{
		ID:        id,
		LastLogin: lastLogin,
	})
}
