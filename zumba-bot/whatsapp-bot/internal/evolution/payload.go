package evolution

// WebhookEvent ist der rohe Body, den die Evolution API per Webhook an den Bot
// schickt (messages.upsert). Felder spiegeln die im n8n-Workflow genutzten Pfade
// – ohne das n8n-eigene "body."-Prefix, da Go den Body direkt empfängt.
type WebhookEvent struct {
	Sender string `json:"sender"` // Instanz-Owner-JID (n8n: body.sender)
	Data   struct {
		MessageType string `json:"messageType"`
		PushName    string `json:"pushName"`
		Key         struct {
			RemoteJid      string `json:"remoteJid"`
			FromMe         bool   `json:"fromMe"`
			Participant    string `json:"participant"`
			ParticipantAlt string `json:"participantAlt"`
		} `json:"key"`
		Message struct {
			Conversation string `json:"conversation"`
		} `json:"message"`
	} `json:"data"`
}

// UserID bildet die n8n-"Edit Fields"-Logik ab:
// fromMe ? sender : data.key.participantAlt
func (e WebhookEvent) UserID() string {
	if e.Data.Key.FromMe {
		return e.Sender
	}
	return e.Data.Key.ParticipantAlt
}

func (e WebhookEvent) UserName() string    { return e.Data.PushName }
func (e WebhookEvent) Message() string     { return e.Data.Message.Conversation }
func (e WebhookEvent) RemoteJid() string   { return e.Data.Key.RemoteJid }
func (e WebhookEvent) MessageType() string { return e.Data.MessageType }
