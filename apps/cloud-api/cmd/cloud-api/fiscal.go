package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/certificados"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/fiscal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/config"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/firma/firmaprueba"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
)

// workerFiscal es el único proceso con la KEK privada (ADR-0006, F5-07).
func workerFiscal(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pemData, err := os.ReadFile(cfg.KEKPrivadaFile)
	if err != nil {
		return fmt.Errorf("worker fiscal: no se pudo leer la KEK privada (%s): %w", cfg.KEKPrivadaFile, err)
	}
	kek, err := certificados.CargarKEKPrivada(pemData)
	if err != nil {
		return err
	}
	d, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer d.Close()
	host, _ := os.Hostname()
	w := &fiscal.Worker{DB: d, KEK: kek, Host: cfg.SRIHost, Politica: sriws.PoliticaPorDefecto, Now: time.Now,
		Log: slog.Default(), Proceso: "fiscal@" + host}
	slog.Info("worker fiscal en marcha", "sri", map[bool]string{true: "SRI según el ambiente", false: cfg.SRIHost}[cfg.SRIHost == ""])
	w.Correr(ctx, d.Pool, 2*time.Second)
	return nil
}

// kekGenerar crea el par de llaves local. No sobrescribe: perder la privada deja ilegibles los
// certificados ya guardados.
func kekGenerar(args []string) error {
	fs := flag.NewFlagSet("kek-generar", flag.ContinueOnError)
	dir := fs.String("dir", ".secrets", "carpeta de destino (fuera del repositorio)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pub, priv := filepath.Join(*dir, "kek-publica.pem"), filepath.Join(*dir, "kek-privada.pem")
	if _, err := os.Stat(priv); err == nil {
		fmt.Println("✓ la KEK ya existe en", *dir)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pubPEM, privPEM, err := certificados.GenerarKEK()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(priv, privPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pub, pubPEM, 0o644); err != nil { //nolint:gosec // es la llave pública
		return err
	}
	fmt.Println("✓ KEK de desarrollo creada en", *dir)
	return nil
}

// p12Prueba crea un certificado ficticio (el SRI real no lo acepta; el stub sí).
func p12Prueba(args []string) error {
	fs := flag.NewFlagSet("p12-prueba", flag.ContinueOnError)
	ruc := fs.String("ruc", "", "RUC del titular")
	esDemo := fs.Bool("demo", false, "usar el RUC del restaurante de demostración")
	titular := fs.String("titular", "FIRMA DE PRUEBA", "nombre del titular")
	clave := fs.String("clave", "", "contraseña del .p12")
	out := fs.String("out", ".secrets/prueba.p12", "archivo de salida")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *esDemo {
		*ruc = rucDemo()
	}
	if *ruc == "" || *clave == "" {
		return errors.New("indica -ruc (o -demo) y -clave")
	}
	ahora := time.Now()
	b, err := firmaprueba.P12(firmaprueba.Opciones{Titular: *titular, RUC: *ruc, Desde: ahora.Add(-time.Hour), Hasta: ahora.AddDate(1, 0, 0)}, *clave)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(*out, b, 0o600); err != nil {
		return err
	}
	fmt.Println("✓ certificado de prueba en", *out)
	return nil
}
