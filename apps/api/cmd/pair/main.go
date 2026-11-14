// Command pair vincula um número ao gateway lendo o QR Code no terminal.
//
// Ele usa o mesmo `session.db` do daemon, então o daemon deve estar PARADO
// durante o pareamento (o SQLite não aceita dois escritores).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"
)

const defaultDSN = "file:./data/session.db?_foreign_keys=on"

func main() {
	invert := flag.Bool("invert", false, "desenha o QR com as cores invertidas (use se o leitor não reconhecer)")
	pngPath := flag.String("png", "", "salva o QR também como PNG no caminho informado")
	flag.Parse()

	if err := run(*invert, *pngPath); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(invert bool, pngPath string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := envOr("GABE_WHATS_SQLITE_DSN", defaultDSN)
	if dir := dsnDir(dsn); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("criar diretório da sessão: %w", err)
		}
	}

	logger := waLog.Stdout("Pair", strings.ToLower(envOr("GABE_WHATS_LOG_LEVEL", "error")), true)

	container, err := sqlstore.New(ctx, "sqlite3", dsn, logger)
	if err != nil {
		return fmt.Errorf("abrir session.db: %w", err)
	}
	defer func() { _ = container.Close() }()

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("recuperar dispositivo: %w", err)
	}
	if device.ID != nil {
		fmt.Printf("Já existe sessão pareada: %s\n", device.ID.String())
		fmt.Println("Nada a fazer. Para trocar de número, faça logout pela API ou apague o session.db.")
		return nil
	}

	client := whatsmeow.NewClient(device, logger)
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("abrir canal de QR: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("conectar: %w", err)
	}
	defer client.Disconnect()

	fmt.Println("Aguardando o primeiro QR Code...")
	fmt.Println("No celular: WhatsApp > Aparelhos conectados > Conectar aparelho")

	codes := 0
	for item := range qrChan {
		switch item.Event {
		case whatsmeow.QRChannelEventCode:
			codes++
			if codes > 1 {
				fmt.Println("\nQR expirou; novo código abaixo.")
			}
			qr, err := qrcode.New(item.Code, qrcode.Low)
			if err != nil {
				return fmt.Errorf("gerar QR: %w", err)
			}
			fmt.Println()
			fmt.Println(qr.ToSmallString(invert))
			fmt.Println("Escaneie o código acima. Ele é renovado automaticamente até o pareamento.")

			if pngPath != "" {
				if err := qr.WriteFile(256, pngPath); err != nil {
					fmt.Fprintf(os.Stderr, "aviso: não foi possível salvar o PNG: %v\n", err)
				} else {
					fmt.Printf("PNG atualizado em %s\n", pngPath)
				}
			}
		case "success":
			fmt.Println()
			if client.Store.ID != nil {
				fmt.Printf("Pareado com sucesso: %s\n", client.Store.ID.String())
			} else {
				fmt.Println("Pareado com sucesso.")
			}
			fmt.Println("Agora suba o daemon: go run ./apps/api/cmd/server")
			return nil
		case "timeout":
			return errors.New("tempo esgotado: a conexão encerrou antes do pareamento")
		default:
			return fmt.Errorf("evento inesperado do canal de QR: %s", item.Event)
		}
	}

	return errors.New("canal de QR encerrado sem pareamento")
}

// envOr devolve a variável de ambiente ou o padrão.
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// dsnDir extrai o diretório de um DSN SQLite baseado em arquivo.
func dsnDir(dsn string) string {
	path := strings.TrimPrefix(dsn, "file:")
	if index := strings.Index(path, "?"); index >= 0 {
		path = path[:index]
	}
	if path == "" || path == ":memory:" {
		return ""
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}
