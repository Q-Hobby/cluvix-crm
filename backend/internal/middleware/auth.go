package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/seclog"
	"erpCluvix/pkg/token"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

var (
	jwtSecret   []byte
	jwtIssuer   string
	jwtAudience string
)

// SetJWTVerifier đặt khoá + issuer + audience mà JWTAuth đòi ở mọi access token. issuer/audience
// chứa DB_NAME (config.TokenIssuer/TokenAudience) ⇒ token ký đúng TOKEN_SECRET nhưng phát cho khách
// khác (DB khác) bị 401 (R-1). Thiếu issuer/audience ⇒ JWTAuth từ chối MỌI token (fail-closed).
func SetJWTVerifier(secret, issuer, audience string) {
	jwtSecret = []byte(secret)
	jwtIssuer = issuer
	jwtAudience = audience
}

// Claims khớp với JWT do pkg/token issue: uid + ipHash (khi TOKEN_IP_BINDING bật).
// KHÔNG khai báo company_id/username — token chưa bao giờ chứa chúng. Phạm vi dữ liệu (chi
// nhánh) lấy từ UserScope (AuthResponseHeaders); story-41 (E6/D15): không còn phạm vi company.
type Claims struct {
	jwt.RegisteredClaims
	UID    uint   `json:"uid"`
	IPHash string `json:"ipHash,omitempty"`
}

// JWTAuth verifies the Bearer access token: signature + expiry + issuer/audience + jti revocation.
// tokenSvc cung cấp blacklist check (Revoke/RevokeAllSessions ghi `revoked:{jti}`
// vào Redis khi logout-all / đổi mật khẩu / đổi role / enforce session limit) —
// thiếu check này thì access token đã revoke vẫn dùng được tới hết TTL.
func JWTAuth(tokenSvc *token.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Missing authentication token."})
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims := &Claims{}
		parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})
		// Issuer/audience định danh khách (DB_NAME) — chặn token chéo khách chung TOKEN_SECRET (R-1).
		if err != nil || !parsed.Valid || !token.ClaimsMatchTenant(&claims.RegisteredClaims, jwtIssuer, jwtAudience) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token."})
			return
		}

		if claims.ID != "" && tokenSvc != nil && tokenSvc.IsRevoked(claims.ID) {
			// Token đã revoke nhưng còn hạn được tái sử dụng — tín hiệu token theft
			// đáng chú ý (đặc biệt sau logout-all/đổi mật khẩu).
			c.Set("userID", claims.UID) // để seclog đính user_id
			seclog.Event(c, "revoked_token_reused", zap.String("jti", claims.ID))
			// code: mobile app phân biệt revoked (→ remote wipe local data) vs
			// expired thông thường (→ thử refresh). Additive — web ignore.
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token.", "code": "token_revoked"})
			return
		}

		// IP binding (opt-in qua TOKEN_IP_BINDING): token có ipHash → so với IP hiện tại.
		// Token issue khi flag tắt không có ipHash → bỏ qua (backward-compatible).
		if claims.IPHash != "" && claims.IPHash != sha256Hex(c.ClientIP()) {
			c.Set("userID", claims.UID)
			seclog.Event(c, "access_ip_mismatch")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token."})
			return
		}

		c.Set("userID", claims.UID)
		c.Set("logger", logger.L.With(zap.Uint("user_id", claims.UID)))
		c.Next()
	}
}

// sha256Hex khớp cách pkg/token băm IP (sha256 hex) — để so ipHash trong claims.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func GetUserID(c *gin.Context) uint {
	v, _ := c.Get("userID")
	if id, ok := v.(uint); ok {
		return id
	}
	return 0
}

// GetCompanyID ĐÃ XÓA (2026-07-04): JWT không chứa company_id nên hàm luôn trả 0.
// Rule "cấm GetCompanyID trong business logic" giờ enforce bằng compile error.
// Story-41 (E6/D15): mọi helper đọc company từ session cũng đã xoá — 1 thương
// hiệu/DB, phạm vi dữ liệu chỉ còn chi nhánh (GetBranchActiveID/GetBranchIDs).

func GetBranchActiveID(c *gin.Context) uint {
	v, _ := c.Get("branchActiveId")
	if id, ok := v.(uint); ok {
		return id
	}
	return 0
}

func GetBranchIDs(c *gin.Context) []uint {
	v, _ := c.Get("branchIds")
	if ids, ok := v.([]uint); ok {
		return ids
	}
	return nil
}

// IsBranchAllowed kiểm tra branchID (lấy tường minh từ body/query khi client chọn branch)
// có nằm trong quyền hạn branch của user hiện tại (GetBranchIDs) hay không. Bắt buộc gọi
// trước khi ghi record theo branchID tường minh từ client — thiếu bước này client có thể
// gắn record vào branch không thuộc quyền (branch_id là global cross-company) → dữ liệu sai
// + né guard theo branch (vd khóa kỳ). branchIds rỗng = user không giới hạn branch → cho qua.
func IsBranchAllowed(c *gin.Context, branchID uint) bool {
	if branchID == 0 {
		return false
	}
	ids := GetBranchIDs(c)
	if len(ids) == 0 {
		return true
	}
	for _, id := range ids {
		if id == branchID {
			return true
		}
	}
	return false
}

// GetLogger returns the per-request logger enriched with user_id and username.
// Falls back to the global logger when called outside an authenticated context.
func GetLogger(c *gin.Context) *zap.Logger {
	v, _ := c.Get("logger")
	if l, ok := v.(*zap.Logger); ok {
		return l
	}
	return logger.L
}
