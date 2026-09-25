package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/personal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/tenants"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// rucDemo arma un RUC de persona natural sintético y válido (no pertenece a nadie real).
func rucDemo() string {
	for d := '0'; d <= '9'; d++ {
		ced := "091357924" + string(d)
		if sri.ValidarCedula(ced).Valida {
			return ced + "001"
		}
	}
	panic("sin RUC demo")
}

type platoDemo struct {
	categoria, nombre, alias, desc, precio, foto string
	grupos                                       []string
}

// demo crea «Cevichería Don Pepe» con menú fotografiado, salón y equipo: sirve para la
// demostración «anti-caídas» en ventas (docs/09 §3) y para desarrollar el backoffice.
func demo(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	email := fs.String("email", "demo@donpepe.ec", "correo del dueño del restaurante demo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	app, err := build(ctx, true)
	if err != nil {
		return err
	}
	defer app.DB.Close()
	if n, err := app.Imagenes.SembrarGaleria(ctx); err != nil {
		return fmt.Errorf("galería: %w (¿está arriba `make dev`?)", err)
	} else if n > 0 {
		fmt.Printf("✓ galería: %d fotos subidas\n", n)
	}

	r, err := app.Tenants.Crear(ctx, tenants.Alta{
		RUC: rucDemo(), RazonSocial: "José Andrade (demo)", NombreComercial: "Cevichería Don Pepe",
		NombreDueno: "Pepe Andrade", EmailDueno: *email, Plan: "RESTAURANTE",
	})
	if err != nil {
		return err
	}
	// El demo entra directo con una contraseña conocida (solo desarrollo y ventas).
	const clave = "DonPepe2026"
	h, _ := auth.HashPassword(clave)
	if err := app.DB.InTenant(ctx, r.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash=$2, debe_cambiar_password=false WHERE id=$1`, r.UsuarioID, h)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE locales SET direccion='Malecón 2000 y Av. 9 de Octubre, Guayaquil', propina_legal_activa=true WHERE id=$1`, r.LocalID)
		return err
	}); err != nil {
		return err
	}
	p := auth.Principal{TenantID: r.TenantID, UserID: r.UsuarioID}

	// Categorías: se reutilizan las sembradas y se agregan las del demo.
	cats, err := app.Catalogo.Categorias(ctx, p)
	if err != nil {
		return err
	}
	catID := map[string]ids.ID{}
	for _, c := range cats {
		catID[c.Nombre] = c.ID
	}
	nuevas := []catalogo.CategoriaInput{
		{Nombre: "Mariscos", Icono: "fish", Color: "teal", EstacionID: &r.EstacionCocina, Notas: []string{"Sin cebolla", "Extra limón", "Poco picante", "Con canguil"}},
		{Nombre: "Desayunos", Icono: "egg-fried", Color: "yellow", EstacionID: &r.EstacionCocina, Notas: []string{"Huevos revueltos", "Huevos fritos"}},
		{Nombre: "Rápidas", Icono: "sandwich", Color: "red", EstacionID: &r.EstacionCocina, Notas: []string{"Sin mayonesa", "Extra queso"}},
		{Nombre: "Postres", Icono: "cake-slice", Color: "pink", EstacionID: &r.EstacionBar},
	}
	for _, c := range nuevas {
		cr, err := app.Catalogo.CrearCategoria(ctx, p, c)
		if err != nil {
			return err
		}
		catID[cr.Nombre] = cr.ID
	}

	grupos := map[string]ids.ID{}
	for _, g := range []catalogo.GrupoInput{
		{Nombre: "Término de la carne", Obligatorio: true, Min: 1, Max: 1, Modificadores: []catalogo.ModificadorInput{{Nombre: "Término medio"}, {Nombre: "Tres cuartos"}, {Nombre: "Bien cocida"}}},
		{Nombre: "Extras", Max: 3, Modificadores: []catalogo.ModificadorInput{{Nombre: "Tocino", PrecioAdicional: "1.00"}, {Nombre: "Queso extra", PrecioAdicional: "0.75"}, {Nombre: "Huevo frito", PrecioAdicional: "0.60"}}},
		{Nombre: "Acompañado", Obligatorio: true, Min: 1, Max: 1, Modificadores: []catalogo.ModificadorInput{{Nombre: "Arroz"}, {Nombre: "Patacones"}, {Nombre: "Chifles"}}},
	} {
		gr, err := app.Catalogo.CrearGrupo(ctx, p, g)
		if err != nil {
			return err
		}
		grupos[gr.Nombre] = gr.ID
	}

	tarifas, err := app.Catalogo.TarifasVigentes(ctx)
	if err != nil || len(tarifas) == 0 {
		return fmt.Errorf("sin tarifas de IVA: %w", err)
	}
	iva15 := tarifas[0].ID // la vigente más alta (15 %)

	platos := []platoDemo{
		{"Mariscos", "Ceviche de camarón", "CC", "Camarón fresco con limón, cebolla colorada y culantro. Con canguil y chifles.", "12.50", "ceviche-camaron", []string{"Acompañado"}},
		{"Mariscos", "Ceviche de pescado", "CP", "Pescado del día marinado al limón con choclo y camote.", "11.00", "ceviche-pescado", []string{"Acompañado"}},
		{"Mariscos", "Arroz marinero", "AM", "Arroz con camarón, calamar y mejillones.", "14.00", "arroz-marinero", nil},
		{"Mariscos", "Pescado frito", "PF", "Corvina entera frita con arroz, ensalada y patacones.", "13.50", "pescado-frito", nil},
		{"Platos fuertes", "Seco de pollo", "SP", "Pollo criollo en salsa de naranjilla y cerveza, con arroz amarillo.", "8.50", "seco-pollo", nil},
		{"Platos fuertes", "Parrillada", "PA", "Pollo a la plancha con tomate asado.", "12.00", "parrillada", []string{"Término de la carne", "Acompañado"}},
		{"Platos fuertes", "Pasta a la marinera", "PM", "Tallarín en salsa de tomate con mariscos.", "10.50", "pasta", nil},
		{"Desayunos", "Bolón mixto", "BM", "Bolón de verde con queso y chicharrón, huevo y café.", "5.50", "bolon", []string{"Extras"}},
		{"Desayunos", "Desayuno completo", "DC", "Huevos, tostadas, salchicha y jugo natural.", "6.00", "desayuno", nil},
		{"Entradas", "Empanadas de verde", "EV", "Tres empanadas rellenas de queso.", "4.50", "empanadas", nil},
		{"Entradas", "Alitas BBQ", "AB", "Ocho alitas bañadas en salsa BBQ de la casa.", "7.50", "alitas", nil},
		{"Entradas", "Ensalada fresca", "EF", "Hojas verdes, tomate cherry y pepino.", "5.00", "ensalada", nil},
		{"Rápidas", "Hamburguesa Don Pepe", "HB", "Doble carne, queso cheddar, tocino y ensalada de col.", "9.50", "hamburguesa", []string{"Término de la carne", "Extras"}},
		{"Rápidas", "Pizza personal", "PZ", "Jamón, queso mozzarella y aceitunas.", "8.00", "pizza", []string{"Extras"}},
		{"Rápidas", "Papas fritas", "PAP", "Porción grande con salsas.", "3.00", "papas-fritas", nil},
		{"Postres", "Tres leches", "TL", "Bizcocho húmedo con crema y fresa.", "3.50", "tres-leches", nil},
		{"Postres", "Torta de chocolate", "TC", "Porción de torta de chocolate con crema.", "3.75", "torta-chocolate", nil},
		{"Bebidas", "Jugo natural", "JN", "Maracuyá, mora o naranjilla.", "2.50", "jugo", nil},
		{"Bebidas", "Limonada", "LI", "Limonada fresca endulzada con panela.", "2.00", "limonada", nil},
		{"Bebidas", "Cerveza", "CE", "Cerveza nacional bien fría.", "3.00", "cerveza", nil},
		{"Bebidas", "Café pasado", "CF", "Café de Loja.", "1.75", "cafe", nil},
		{"Bebidas", "Mojito", "MO", "Ron, hierbabuena y limón.", "6.00", "coctel", nil},
	}
	for _, pl := range platos {
		key := "biblioteca/" + pl.foto
		alias := pl.alias
		in := catalogo.ProductoInput{CategoriaID: catID[pl.categoria], Nombre: pl.nombre, Alias: &alias, Descripcion: pl.desc, Precio: pl.precio, TarifaIVAID: iva15, ImagenKey: &key}
		for _, g := range pl.grupos {
			in.Grupos = append(in.Grupos, grupos[g])
		}
		if _, err := app.Catalogo.CrearProducto(ctx, p, in); err != nil {
			return fmt.Errorf("%s: %w", pl.nombre, err)
		}
	}

	if _, err := app.Salon.CrearLote(ctx, p, r.ZonaID, 11, 4); err != nil {
		return err
	}
	terraza, err := app.Salon.CrearZona(ctx, p, salon.ZonaInput{LocalID: r.LocalID, Nombre: "Terraza"})
	if err != nil {
		return err
	}
	if _, err := app.Salon.CrearLote(ctx, p, terraza.ID, 6, 2); err != nil {
		return err
	}

	equipo := []personal.UsuarioInput{
		{NombreMostrar: "Carlos M.", Rol: "MESERO", PIN: "8899"},
		{NombreMostrar: "Ana R.", Rol: "MESERO", PIN: "1024"},
		{NombreMostrar: "Luis P.", Rol: "CAJERO", PIN: "7391"},
		{NombreMostrar: "María C.", Rol: "COCINA", PIN: "5821"},
	}
	for _, u := range equipo {
		if _, err := app.Personal.Crear(ctx, p, u); err != nil {
			return fmt.Errorf("%s: %w", u.NombreMostrar, err)
		}
	}

	fmt.Printf(`✓ Restaurante demo listo: Cevichería Don Pepe
  Panel:      %s/login
  Usuario:    %s
  Contraseña: %s
  Menú:       %d platos con foto · 18 mesas en Salón y Terraza
  Equipo:     Carlos 8899 · Ana 1024 · Luis (caja) 7391 · María (cocina) 5821
`, app.Cfg.BackofficeURL, *email, clave, len(platos))
	return nil
}
