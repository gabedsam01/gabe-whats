package contracts

import "time"

// SessionStatus resume o estado da conexão do cliente WhatsApp pessoal.
type SessionStatus struct {
	Connected     bool   `json:"connected" doc:"Se o WebSocket com o servidor do WhatsApp está conectado"`
	LoggedIn      bool   `json:"logged_in" doc:"Se o dispositivo está autenticado e pareado com o número"`
	JID           string `json:"jid,omitempty" doc:"JID do número conectado (ex: 5511999999999@s.whatsapp.net)"`
	PushName      string `json:"push_name,omitempty" doc:"Nome de exibição (push name) do número conectado"`
	Platform      string `json:"platform,omitempty" doc:"Plataforma reportada pelo pareamento (ex: smba, android)"`
	UptimeSeconds int64  `json:"uptime_seconds" doc:"Tempo em segundos desde o início da conexão atual"`
}

// QRCodeResponse entrega o QR Code de pareamento ou o resultado do ciclo de conexão.
type QRCodeResponse struct {
	Event     string     `json:"event" enum:"code,success,timeout" doc:"Evento do canal de QR: code (novo código), success (pareado) ou timeout (código expirado)"`
	Code      string     `json:"code,omitempty" doc:"Conteúdo bruto do QR Code, útil para exibição em terminal"`
	PNGBase64 string     `json:"png_base64,omitempty" doc:"Imagem do QR Code codificada em Base64 (PNG) para renderização pelo agente"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" doc:"Instante em que o QR Code expira; ausente em eventos success e timeout"`
}

// PairCodeRequest solicita o pareamento por código de 8 dígitos (PairPhone).
type PairCodeRequest struct {
	Phone string `json:"phone" required:"true" doc:"Número que será pareado, com DDI e apenas dígitos (ex: 5511999999999)"`
}

// PairCodeResponse devolve o código de pareamento para digitar no WhatsApp.
type PairCodeResponse struct {
	Code      string    `json:"code" doc:"Código de 8 dígitos exibido em WhatsApp > Dispositivos conectados"`
	ExpiresAt time.Time `json:"expires_at" doc:"Instante em que o código expira e precisa ser gerado novamente"`
}
