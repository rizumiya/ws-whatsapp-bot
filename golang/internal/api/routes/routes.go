package routes

import (
	"golang-whatsapp-bot/internal/api/handlers"
	"golang-whatsapp-bot/internal/api/middlewares"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Session Endpoints
	sessions := api.Group("/sessions")
	sessions.Get("/", middlewares.MasterAuth(), handlers.GetSessions)
	sessions.Post("/", middlewares.MasterAuth(), handlers.CreateSession)

	sessionID := sessions.Group("/:sessionId", middlewares.SessionAuth())
	sessionID.Get("/status", handlers.GetSessionStatus)
	sessionID.Get("/qr", handlers.GetQR)
	sessionID.Post("/pairing-code", handlers.GetPairingCode)
	sessionID.Post("/restart", handlers.RestartSession)
	sessionID.Delete("/logout", handlers.LogoutSession)

	// Messaging Endpoints
	sessionID.Post("/messages/text", handlers.SendText)
	sessionID.Post("/messages/media", handlers.SendMedia)
	sessionID.Post("/messages/location", handlers.SendLocation)
	sessionID.Post("/messages/react", handlers.SendReaction)
	sessionID.Post("/messages/revoke", handlers.RevokeMessage)
	sessionID.Get("/messages/:messageId/download", handlers.DownloadMedia)

	// Groups
	groups := sessionID.Group("/groups")
	groups.Post("/", handlers.CreateGroup)
	groups.Get("/:jid", handlers.GetGroupInfo)
	groups.Post("/:jid/participants/add", handlers.AddParticipants)
	groups.Post("/:jid/participants/remove", handlers.RemoveParticipants)
	groups.Post("/:jid/participants/promote", handlers.PromoteParticipants)
	groups.Post("/:jid/participants/demote", handlers.DemoteParticipants)
	groups.Patch("/:jid/subject", handlers.UpdateGroupSubject)
	groups.Patch("/:jid/description", handlers.UpdateGroupDescription)
	groups.Get("/:jid/invite", handlers.GetGroupInviteLink)

	// Chats
	chats := sessionID.Group("/chats")
	chats.Post("/read", handlers.MarkRead)
	chats.Post("/presence", handlers.UpdatePresence)

	// Calls
	calls := sessionID.Group("/calls")
	calls.Post("/reject", handlers.RejectCall)
	calls.Post("/accept", handlers.AcceptCall)
	calls.Post("/offer", handlers.OfferCall)
}
