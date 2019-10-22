package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/models/payments"
)

// GetAllPayments finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) GetAllPayments() gin.HandlerFunc {
	type Params struct {
		Limit  int `form:"limit" binding:"gte=0"`
		Offset int `form:"offset" binding:"gte=0"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var params Params
		if c.BindQuery(&params) != nil {
			return
		}

		t, err := payments.GetAll(r.db, userID, params.Limit, params.Offset)
		if err != nil {
			log.WithError(err).Errorf("Couldn't get payments")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// GetPaymentByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetPaymentByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		log.Infof("find payment %d for user %d", id, userID)

		t, err := payments.GetByID(r.db, int(id), userID)
		if err != nil {
			apierr.Public(c, http.StatusNotFound, apierr.ErrInvoiceNotFound)
			return
		}

		log.Infof("found payment %v", t)

		// Return the user when it is found and no errors where encountered
		c.JSONP(http.StatusOK, t)
	}
}

// CreateInvoice creates a new invoice
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
	// CreateInvoiceRequest is a deposit
	type CreateInvoiceRequest struct {
		AmountSat   int64  `json:"amountSat" binding:"required,gt=0,lte=4294967"`
		Memo        string `json:"memo" binding:"max=256"`
		Description string `json:"description"`
		CallbackURL string `json:"callbackUrl" binding:"omitempty,url"`
		OrderId     string `json:"orderId" binding:"max=256"`
	}

	return func(c *gin.Context) {

		userID, ok := getUserIdOrReject(c)

		if !ok {
			return
		}

		var req CreateInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		t, err := payments.NewPayment(
			r.db, r.lncli, payments.NewPaymentOpts{
				UserID:      userID,
				AmountSat:   req.AmountSat,
				Memo:        req.Memo,
				Description: req.Description,
				CallbackURL: req.CallbackURL,
				OrderId:     req.OrderId,
			})

		if err != nil {
			log.WithError(err).Error("Could not add new payment")
			_ = c.Error(err)
			return
		}

		if t.UserID != userID {
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func (r *RestServer) PayInvoice() gin.HandlerFunc {
	// PayInvoiceRequest is the required and optional fields for initiating a
	// withdrawal.
	type PayInvoiceRequest struct {
		PaymentRequest string `json:"paymentRequest" binding:"required,paymentrequest"`
		Description    string `json:"description"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req PayInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoiceWithDescription(r.db, r.lncli, userID,
			req.PaymentRequest, req.Description)
		if err != nil {
			// investigate details around failure
			go func() {
				origErr := err
				decoded, err := r.lncli.DecodePayReq(context.Background(), &lnrpc.PayReqString{
					PayReq: req.PaymentRequest,
				})
				if err != nil {
					log.WithError(err).Error("Could not decode payment request")
					return
				}

				channels, err := r.lncli.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
					ActiveOnly: true,
				})
				if err != nil {
					log.WithError(err).Error("Could not list active channels")
					return
				}

				balance, err := r.lncli.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
				if err != nil {
					log.WithError(err).Error("Could not get channel balance")
				}

				log.WithFields(logrus.Fields{
					"destination":    decoded.Destination,
					"amountSat":      decoded.NumSatoshis,
					"activeChannels": len(channels.Channels),
					"channelBalance": balance.Balance,
					"routeHints":     decoded.RouteHints,
				}).WithError(origErr).Error("Could not pay invoice")
				fmt.Println(2 + 2)

			}()
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}
