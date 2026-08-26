package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/config"
	"navigator-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
)

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

const (
	pbkdf2Iterations = 100000
	pbkdf2KeyLen     = 32
	saltLen          = 16
)

func (s *AuthService) HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, pbkdf2KeyLen, sha256.New)
	saltB64 := base64.StdEncoding.EncodeToString(salt)
	hashB64 := base64.StdEncoding.EncodeToString(hash)

	// Format: iterations.saltB64.hashB64
	return fmt.Sprintf("%d.%s.%s", pbkdf2Iterations, saltB64, hashB64), nil
}

func (s *AuthService) VerifyPassword(password, storedHash string) bool {
	parts := strings.Split(storedHash, ".")
	if len(parts) != 3 {
		// Legacy / simple fallback
		return false
	}

	iter, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	salt, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	expectedHash, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	actualHash := pbkdf2.Key([]byte(password), salt, iter, len(expectedHash), sha256.New)
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

func (s *AuthService) GenerateToken(user *models.AppUser) (string, time.Time, error) {
	expiresAt := time.Now().UTC().Add(s.cfg.JwtExpiryDuration())
	claims := jwt.MapClaims{
		"sub":   strconv.Itoa(user.Id),
		"name":  user.Username,
		"email": user.Email,
		"role":  user.Role,
		"iss":   s.cfg.JwtIssuer,
		"aud":   s.cfg.JwtAudience,
		"exp":   expiresAt.Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.JwtSecret))
	return tokenStr, expiresAt, err
}

func (s *AuthService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	normUsername := strings.ToUpper(strings.TrimSpace(req.Username))
	normEmail := strings.ToUpper(strings.TrimSpace(req.Email))

	var existing models.AppUser
	err := s.db.Where("normalized_username = ? OR normalized_email = ?", normUsername, normEmail).First(&existing).Error
	if err == nil {
		return nil, errors.New("username or email is already registered")
	}

	hash, err := s.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	var userCount int64
	_ = s.db.Model(&models.AppUser{}).Count(&userCount).Error

	role := "User"
	if userCount == 0 {
		role = "Admin"
	}

	user := models.AppUser{
		Username:           strings.TrimSpace(req.Username),
		NormalizedUsername: normUsername,
		Email:              strings.TrimSpace(req.Email),
		NormalizedEmail:    normEmail,
		PasswordHash:       hash,
		Role:               role,
		CreatedAtUtc:       time.Now().UTC(),
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	token, expiresAt, err := s.GenerateToken(&user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token:        token,
		ExpiresAtUtc: expiresAt,
		User: models.AuthUserDto{
			Id:       user.Id,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}, nil
}

func (s *AuthService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	normQuery := strings.ToUpper(strings.TrimSpace(req.UsernameOrEmail))

	var user models.AppUser
	err := s.db.Where("normalized_username = ? OR normalized_email = ?", normQuery, normQuery).First(&user).Error
	if err != nil {
		return nil, errors.New("invalid username/email or password")
	}

	if !s.VerifyPassword(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid username/email or password")
	}

	token, expiresAt, err := s.GenerateToken(&user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token:        token,
		ExpiresAtUtc: expiresAt,
		User: models.AuthUserDto{
			Id:       user.Id,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}, nil
}

func (s *AuthService) GetUserById(id int) (*models.AppUser, error) {
	var user models.AppUser
	err := s.db.First(&user, id).Error
	return &user, err
}

func (s *AuthService) GetAllUsers() ([]models.AuthUserDto, error) {
	var users []models.AppUser
	err := s.db.Order("username asc").Find(&users).Error
	if err != nil {
		return nil, err
	}

	res := make([]models.AuthUserDto, len(users))
	for i, u := range users {
		res[i] = models.AuthUserDto{
			Id:       u.Id,
			Username: u.Username,
			Email:    u.Email,
			Role:     u.Role,
		}
	}
	return res, nil
}

func (s *AuthService) UpdateUserRole(userId int, role string) (*models.AuthUserDto, error) {
	if !strings.EqualFold(role, "Admin") && !strings.EqualFold(role, "User") {
		return nil, errors.New("role must be 'Admin' or 'User'")
	}

	formattedRole := "User"
	if strings.EqualFold(role, "Admin") {
		formattedRole = "Admin"
	}

	var user models.AppUser
	if err := s.db.First(&user, userId).Error; err != nil {
		return nil, fmt.Errorf("user %d was not found", userId)
	}

	user.Role = formattedRole
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &models.AuthUserDto{
		Id:       user.Id,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}
