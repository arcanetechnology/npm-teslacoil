package apikeyroutes

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
)

var log = build.Log

var database *db.DB

func RegisterRoutes(server *gin.Engine, db *db.DB, authmiddleware gin.HandlerFunc) *gin.RouterGroup {
	database = db

	keys := server.Group("apikey")
	keys.Use(authmiddleware)
	keys.POST("", createApiKey())

	// Gin uses a simple but fast routing lib (httprouter), that doesn't handle
	// advanced (ish) routes. so sadly, this doesn't work
	// keys.GET("/all", getAllForUser())
	// keys.GET("/:hash", getByHash())
	// because Gin isn't able to differentiate between the two routes

	keys.GET("/:hash", getAllOrByHash())

	return keys
}

// see the comment in RegisterRoutes about why this is done in a somewhat hacky
// manner
func getAllOrByHash() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("hash") == "all" {
			getAllForUser(c)
		} else {
			getByHash(c)
		}
	}
}

func getByHash(c *gin.Context) {
	type request struct {
		Hash string `uri:"hash" binding:"required,hexadecimal"`
	}

	userId, ok := auth.RequireScope(c, auth.ReadWallet)
	if !ok {
		return
	}

	var req request
	if c.BindUri(&req) != nil {
		return
	}

	hash, err := hex.DecodeString(req.Hash)
	// this shouldn't happen, as the `hexadecimal` validation tag above should
	// make sure we have valid hex
	if err != nil {
		log.WithError(err).Error("Could not decode request hash into bytes")
		_ = c.Error(err)
		return
	}

	key, err := apikeys.GetByHash(database, userId, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.Public(c, http.StatusNotFound, apierr.ErrApiKeyNotFound)
			return
		}
		log.WithError(err).Error("Could not get API key by hash")
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, key)
}

func getAllForUser(c *gin.Context) {
	id, ok := auth.RequireScope(c, auth.ReadWallet)
	if !ok {
		return
	}

	keys, err := apikeys.GetByUserId(database, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, keys)
}

func createApiKey() gin.HandlerFunc {
	type createApiKeyResponse struct {
		Key    uuid.UUID `json:"key"`
		UserID int       `json:"userId"`
		apikeys.Permissions
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.ReadWallet)
		if !ok {
			return
		}

		var perm apikeys.Permissions
		if c.BindJSON(&perm) != nil {
			return
		}

		rawKey, key, err := apikeys.New(database, userID, perm)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("could not create API key")
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusCreated, createApiKeyResponse{
			Key:         rawKey,
			UserID:      key.UserID,
			Permissions: key.Permissions,
		})
	}
}
