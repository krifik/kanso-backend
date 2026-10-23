package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"kanso/backend/internal/infrastructure/config"
	"kanso/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GoogleSSOService interface {
	GetAuthURL() string
	ExchangeCode(ctx context.Context, code string) (*GoogleUserInfo, error)
	IsConnected() bool
	GetConnectedUser() *GoogleUserInfo
	GetConnectedAccounts() []GoogleUserInfo
	FetchRecentMutationEmails(ctx context.Context, days int, maxResults int, customRange ...EmailDateRange) ([]ParsedEmailMessage, error)
	FetchRecentMutationEmailsForAccount(ctx context.Context, email string, days int, maxResults int, customRange ...EmailDateRange) ([]ParsedEmailMessage, error)
	Disconnect(ctx context.Context) error
	DisconnectAccount(ctx context.Context, email string) error
}

type EmailDateRange struct {
	AfterDate  *time.Time
	BeforeDate *time.Time
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

type ParsedEmailMessage struct {
	ID           string
	Subject      string
	Sender       string
	Body         string
	HTMLBody     string
	Date         time.Time
	AccountEmail string
}

type googleSSOService struct {
	cfg   config.Config
	db    *gorm.DB
	mu    sync.RWMutex
	user  *GoogleUserInfo
}

func NewGoogleSSOService(cfg config.Config, db *gorm.DB) GoogleSSOService {
	s := &googleSSOService{
		cfg: cfg,
		db:  db,
	}
	// Restore existing persistent session from DB on startup
	s.restoreSessionFromDB()
	return s
}

func (s *googleSSOService) restoreSessionFromDB() {
	if s.db == nil {
		return
	}
	var sess model.UserOAuthSession
	if err := s.db.Where("provider = ?", "google").Order("updated_at desc").First(&sess).Error; err == nil {
		s.user = &GoogleUserInfo{
			Email:   sess.UserEmail,
			Name:    sess.UserName,
			Picture: sess.AvatarURL,
		}
	}
}

func (s *googleSSOService) getValidToken(ctx context.Context, email ...string) (string, string, error) {
	if s.db == nil {
		return "", "", errors.New("database not configured")
	}

	var sess model.UserOAuthSession
	query := s.db.Where("provider = ?", "google")
	if len(email) > 0 && email[0] != "" {
		query = query.Where("user_email = ?", email[0])
	}
	if err := query.Order("updated_at desc").First(&sess).Error; err != nil {
		return "", "", errors.New("tidak ada sesi Google yang aktif. Silakan hubungkan akun Google Anda.")
	}

	// If token has more than 2 minutes left, use it
	if time.Now().Add(2 * time.Minute).Before(sess.TokenExpiry) {
		return sess.AccessToken, sess.UserEmail, nil
	}

	// If expired but refresh token exists, refresh it
	if sess.RefreshToken != "" {
		newToken, expiresIn, err := s.refreshAccessToken(ctx, sess.RefreshToken)
		if err == nil {
			sess.AccessToken = newToken
			sess.TokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
			sess.UpdatedAt = time.Now()
			_ = s.db.Save(&sess)
			return newToken, sess.UserEmail, nil
		}
	}

	// Fallback to current access token if refresh not possible
	return sess.AccessToken, sess.UserEmail, nil
}

func (s *googleSSOService) refreshAccessToken(ctx context.Context, refreshToken string) (string, int, error) {
	tokenURL := "https://oauth2.googleapis.com/token"
	data := url.Values{}
	data.Set("client_id", s.cfg.GoogleClientID)
	data.Set("client_secret", s.cfg.GoogleClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("token refresh failed: %s", string(b))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", 0, err
	}

	return res.AccessToken, res.ExpiresIn, nil
}

func (s *googleSSOService) GetAuthURL() string {
	baseURL := "https://accounts.google.com/o/oauth2/v2/auth"
	params := url.Values{}
	params.Add("client_id", s.cfg.GoogleClientID)
	params.Add("redirect_uri", s.cfg.GoogleRedirectURL)
	params.Add("response_type", "code")
	params.Add("scope", "https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile")
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")

	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

func (s *googleSSOService) ExchangeCode(ctx context.Context, code string) (*GoogleUserInfo, error) {
	tokenURL := "https://oauth2.googleapis.com/token"
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.cfg.GoogleClientID)
	data.Set("client_secret", s.cfg.GoogleClientSecret)
	data.Set("redirect_uri", s.cfg.GoogleRedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google token exchange failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenRes struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenRes); err != nil {
		return nil, err
	}

	// Fetch User info
	userReq, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()

	var uInfo GoogleUserInfo
	if err := json.NewDecoder(userResp.Body).Decode(&uInfo); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.user = &uInfo
	s.mu.Unlock()

	// Persist session to Database (Upsert UserOAuthSession)
	if s.db != nil {
		var existing model.UserOAuthSession
		expiryTime := time.Now().Add(time.Duration(tokenRes.ExpiresIn) * time.Second)
		if err := s.db.Where("user_email = ?", uInfo.Email).First(&existing).Error; err == nil {
			existing.AccessToken = tokenRes.AccessToken
			if tokenRes.RefreshToken != "" {
				existing.RefreshToken = tokenRes.RefreshToken
			}
			existing.TokenExpiry = expiryTime
			existing.UserName = uInfo.Name
			existing.AvatarURL = uInfo.Picture
			existing.UpdatedAt = time.Now()
			_ = s.db.Save(&existing)
		} else {
			newSession := model.UserOAuthSession{
				ID:           uuid.NewString(),
				Provider:     "google",
				UserEmail:    uInfo.Email,
				UserName:     uInfo.Name,
				AvatarURL:    uInfo.Picture,
				AccessToken:  tokenRes.AccessToken,
				RefreshToken: tokenRes.RefreshToken,
				TokenExpiry:  expiryTime,
				CoupleID:     "DUO-LOVE",
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			_ = s.db.Create(&newSession)
		}
	}

	return &uInfo, nil
}

func (s *googleSSOService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.user != nil {
		return true
	}
	if s.db != nil {
		var count int64
		s.db.Model(&model.UserOAuthSession{}).Where("provider = ?", "google").Count(&count)
		return count > 0
	}
	return false
}

func (s *googleSSOService) GetConnectedUser() *GoogleUserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.user != nil {
		return s.user
	}
	// Try loading from DB
	if s.db != nil {
		var sess model.UserOAuthSession
		if err := s.db.Where("provider = ?", "google").Order("updated_at desc").First(&sess).Error; err == nil {
			return &GoogleUserInfo{
				Email:   sess.UserEmail,
				Name:    sess.UserName,
				Picture: sess.AvatarURL,
			}
		}
	}
	return nil
}

func (s *googleSSOService) GetConnectedAccounts() []GoogleUserInfo {
	var accounts []GoogleUserInfo
	if s.db != nil {
		var sessions []model.UserOAuthSession
		if err := s.db.Where("provider = ?", "google").Order("updated_at desc").Find(&sessions).Error; err == nil {
			for _, sess := range sessions {
				accounts = append(accounts, GoogleUserInfo{
					Email:   sess.UserEmail,
					Name:    sess.UserName,
					Picture: sess.AvatarURL,
				})
			}
		}
	}
	return accounts
}

func (s *googleSSOService) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	s.user = nil
	s.mu.Unlock()

	if s.db != nil {
		return s.db.Where("provider = ?", "google").Delete(&model.UserOAuthSession{}).Error
	}
	return nil
}

func (s *googleSSOService) DisconnectAccount(ctx context.Context, email string) error {
	s.mu.Lock()
	if s.user != nil && s.user.Email == email {
		s.user = nil
	}
	s.mu.Unlock()

	if s.db != nil {
		return s.db.Where("provider = ? AND user_email = ?", "google", email).Delete(&model.UserOAuthSession{}).Error
	}
	return nil
}

func (s *googleSSOService) FetchRecentMutationEmails(ctx context.Context, days int, maxResults int, customRange ...EmailDateRange) ([]ParsedEmailMessage, error) {
	return s.FetchRecentMutationEmailsForAccount(ctx, "", days, maxResults, customRange...)
}

func (s *googleSSOService) FetchRecentMutationEmailsForAccount(ctx context.Context, targetEmail string, days int, maxResults int, customRange ...EmailDateRange) ([]ParsedEmailMessage, error) {
	token, resolvedEmail, err := s.getValidToken(ctx, targetEmail)
	if err != nil {
		return nil, err
	}

	if maxResults <= 0 || maxResults > 50 {
		maxResults = 20
	}

	keywords := "qris OR jago OR bca OR permata OR mandiri OR bri OR bni OR bsi OR cimb OR danamon OR panin OR btn OR mega OR ocbc OR maybank OR btpn OR jenius OR blu OR seabank OR aladin OR neobank OR superbank OR gopay OR gojek OR ovo OR dana OR shopee OR linkaja OR astrapay OR isaku OR flip OR transfer OR debit OR kredit OR topup"
	var timeFilter string

	if len(customRange) > 0 && (customRange[0].AfterDate != nil || customRange[0].BeforeDate != nil) {
		cr := customRange[0]
		var parts []string
		if cr.AfterDate != nil && !cr.AfterDate.IsZero() {
			// after:YYYY/MM/DD or after:epoch
			parts = append(parts, fmt.Sprintf("after:%d", cr.AfterDate.Unix()))
		}
		if cr.BeforeDate != nil && !cr.BeforeDate.IsZero() {
			// before:YYYY/MM/DD or before:epoch
			parts = append(parts, fmt.Sprintf("before:%d", cr.BeforeDate.Unix()))
		}
		timeFilter = strings.Join(parts, " ")
	} else {
		if days <= 0 {
			days = 7
		}
		timeFilter = fmt.Sprintf("newer_than:%dd", days)
	}

	fullQuery := strings.TrimSpace(fmt.Sprintf("(%s) %s", keywords, timeFilter))
	listURL := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=%d&q=%s", maxResults, url.QueryEscape(fullQuery))

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gagal mengambil pesan Gmail (%d): %s", resp.StatusCode, string(b))
	}

	var listData struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listData); err != nil {
		return nil, err
	}

	var results []ParsedEmailMessage

	for _, m := range listData.Messages {
		msgURL := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s?format=full", m.ID)
		msgReq, _ := http.NewRequestWithContext(ctx, "GET", msgURL, nil)
		msgReq.Header.Set("Authorization", "Bearer "+token)

		msgResp, err := client.Do(msgReq)
		if err != nil || msgResp.StatusCode != http.StatusOK {
			continue
		}

		type MessagePart struct {
			MimeType string        `json:"mimeType"`
			Body     struct {
				Data string `json:"data"`
			} `json:"body"`
			Parts    []MessagePart `json:"parts"`
		}

		var fullMsg struct {
			Snippet      string `json:"snippet"`
			InternalDate string `json:"internalDate"`
			Payload      struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
				Body struct {
					Data string `json:"data"`
				} `json:"body"`
				Parts []MessagePart `json:"parts"`
			} `json:"payload"`
		}

		if err := json.NewDecoder(msgResp.Body).Decode(&fullMsg); err == nil {
			var subject, sender, dateHeader string
			for _, h := range fullMsg.Payload.Headers {
				if strings.EqualFold(h.Name, "Subject") {
					subject = h.Value
				} else if strings.EqualFold(h.Name, "From") {
					sender = h.Value
				} else if strings.EqualFold(h.Name, "Date") {
					dateHeader = h.Value
				}
			}

			// Parse real email date
			msgDate := time.Now()
			if dateHeader != "" {
				// Try RFC1123Z, RFC1123, RFC822, RFC822Z, and common variations
				parsedDate, err := http.ParseTime(dateHeader)
				if err == nil {
					msgDate = parsedDate
				} else {
					formats := []string{
						time.RFC1123Z,
						time.RFC1123,
						time.RFC822Z,
						time.RFC822,
						"Mon, 2 Jan 2006 15:04:05 -0700",
						"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
						"2 Jan 2006 15:04:05 -0700",
						"Mon, 2 Jan 2006 15:04:05 MST",
					}
					for _, fmtStr := range formats {
						if pd, err := time.Parse(fmtStr, strings.TrimSpace(dateHeader)); err == nil {
							msgDate = pd
							break
						}
					}
				}
			} else if fullMsg.InternalDate != "" {
				// Fallback to internalDate (epoch in milliseconds)
				var ms int64
				if _, err := fmt.Sscanf(fullMsg.InternalDate, "%d", &ms); err == nil && ms > 0 {
					msgDate = time.UnixMilli(ms)
				}
			}

			var bodyText, htmlText string
			if fullMsg.Payload.Body.Data != "" {
				if decoded, err := base64.URLEncoding.DecodeString(fullMsg.Payload.Body.Data); err == nil {
					bodyText = string(decoded)
					if strings.Contains(bodyText, "<html") || strings.Contains(bodyText, "<body") || strings.Contains(bodyText, "<div") {
						htmlText = bodyText
					}
				}
			}

			// Recursive extractor function for nested parts
			var extractParts func(parts []MessagePart)
			extractParts = func(parts []MessagePart) {
				for _, part := range parts {
					if strings.Contains(part.MimeType, "html") && part.Body.Data != "" && htmlText == "" {
						if decoded, err := base64.URLEncoding.DecodeString(part.Body.Data); err == nil {
							htmlText = string(decoded)
						}
					}
					if strings.Contains(part.MimeType, "plain") && part.Body.Data != "" && bodyText == "" {
						if decoded, err := base64.URLEncoding.DecodeString(part.Body.Data); err == nil {
							bodyText = string(decoded)
						}
					}
					if len(part.Parts) > 0 {
						extractParts(part.Parts)
					}
				}
			}
			extractParts(fullMsg.Payload.Parts)
			if bodyText == "" {
				bodyText = fullMsg.Snippet
			}

			results = append(results, ParsedEmailMessage{
				ID:           m.ID,
				Subject:      subject,
				Sender:       sender,
				Body:         bodyText,
				HTMLBody:     htmlText,
				Date:         msgDate,
				AccountEmail: resolvedEmail,
			})
		}
		msgResp.Body.Close()
	}

	return results, nil
}
