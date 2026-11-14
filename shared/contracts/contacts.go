package contracts

// CheckNumbersRequest verifica quais números possuem conta no WhatsApp.
type CheckNumbersRequest struct {
	Phones []string `json:"phones" required:"true" minItems:"1" maxItems:"100" doc:"Números com DDI, apenas dígitos ou JIDs completos (ex: 5511999999999); até 100 números por requisição"`
}

// CheckNumbersResponse devolve um resultado por número consultado.
type CheckNumbersResponse struct {
	Results []NumberCheckResult `json:"results" doc:"Resultado da verificação, na mesma ordem dos números enviados"`
}

// NumberCheckResult descreve o resultado da consulta de um número.
type NumberCheckResult struct {
	Query        string `json:"query" doc:"Número exatamente como foi enviado na requisição"`
	JID          string `json:"jid,omitempty" doc:"JID resolvido no WhatsApp; vazio quando o número não está registrado"`
	IsRegistered bool   `json:"is_registered" doc:"Se o número possui conta ativa no WhatsApp"`
	PhoneNumber  string `json:"phone_number,omitempty" doc:"Número canônico retornado pelo WhatsApp após a consulta"`
	VerifiedName string `json:"verified_name,omitempty" doc:"Nome verificado da conta comercial, quando existir"`
}
