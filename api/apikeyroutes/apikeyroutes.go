package apikeyroutes

import (
	"database/sql"
	"encoding/base64"
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

var log = build.AddSubLogger("APIK")

var database *db.DB

func RegisterRoutes(server *gin.Engine, db *db.DB, authmiddleware gin.HandlerFunc) *gin.RouterGroup {
	database = db

	keys := server.Group("apikey")
	keys.Use(authmiddleware)
	keys.POST("", createApiKey())
	keys.DELETE("", deleteApiKey())

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
		Hash string `uri:"hash" binding:"required,hexadecimal|urlbase64"`
	}

	userId, ok := auth.RequireScope(c, auth.ReadWallet)
	if !ok {
		return
	}

	var req request
	if c.BindUri(&req) != nil {
		return
	}

	var hash []byte
	if hexhash, err := hex.DecodeString(req.Hash); err == nil {
		hash = hexhash
	} else if b64hash, err := base64.URLEncoding.DecodeString(req.Hash); err == nil {
		hash = b64hash
	} else {
		// this shouldn't happen, as the  validation tags above should
		// make sure we have valid bytes
		err := errors.New("could not decode request hash into bytes")
		log.WithError(err).WithField("hash", req.Hash).Error("Could not decode hex nor base64")
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

func deleteApiKey() gin.HandlerFunc {
	type request struct {
		Hash string `form:"hash" binding:"required,hexadecimal|base64"`
	}

	return func(c *gin.Context) {
		id, ok := auth.RequireScope(c, auth.EditAccount)
		if !ok {
			return
		}

		var req request
		if c.BindQuery(&req) != nil {
			return
		}

		var bytes []byte
		if hexbytes, err := hex.DecodeString(req.Hash); err == nil {
			bytes = hexbytes
		} else if base64bytes, err := base64.StdEncoding.DecodeString(req.Hash); err == nil {
			bytes = base64bytes
		} else {
			err := errors.New("hash was not valid base64 nor hex")
			log.WithError(err).WithField("hash", req.Hash).Error("Reached point we shouldn't reach! " +
				"Hash should have been validated by Gin")
			_ = c.Error(err)
			return
		}

		deleted, err := apikeys.Delete(database, id, bytes)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.Public(c, http.StatusNotFound, apierr.ErrApiKeyNotFound)
				return
			}

			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, deleted)
	}
}

func createApiKey() gin.HandlerFunc {
	type response struct {
		RawKey uuid.UUID `json:"key"`
		apikeys.Key
	}

	type request struct {
		apikeys.Permissions
		Description string `json:"description"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.ReadWallet)
		if !ok {
			return
		}

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		if req.Permissions.IsEmpty() {
			apierr.Public(c, http.StatusBadRequest, apierr.ErrApiKeyNeedPermissons)
			return
		}

		rawKey, key, err := apikeys.New(database, userID, req.Permissions, req.Description)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("could not create API key")
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusCreated, response{
			RawKey: rawKey,
			Key:    key,
		})
	}
}
