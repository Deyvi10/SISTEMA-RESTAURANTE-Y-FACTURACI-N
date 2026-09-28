// Package catalogo administra el menú (RF-03-06, docs/04 §4): categorías con icono de
// comida, productos con foto y precio exacto, modificadores con reglas y notas rápidas.
// La nube es la dueña del catálogo; el Nodo Local lo recibe por sincronización.
package catalogo

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// IconoCategoria es un icono de comida (Lucide) con su nombre para el selector.
type IconoCategoria struct {
	Icono    string `json:"icono"`
	Etiqueta string `json:"etiqueta"`
}

// IconosCategoria: verificados contra lucide@0.460 (no existen «shrimp» ni «hamburger»).
var IconosCategoria = []IconoCategoria{
	{"utensils", "Platos"},
	{"cooking-pot", "Del día"},
	{"chef-hat", "Especiales"},
	{"flame", "Parrilla"},
	{"beef", "Carnes"},
	{"drumstick", "Pollo"},
	{"fish", "Mariscos"},
	{"ham", "Embutidos"},
	{"salad", "Ensaladas"},
	{"soup", "Sopas"},
	{"sandwich", "Sánduches"},
	{"pizza", "Pizzas"},
	{"egg-fried", "Desayunos"},
	{"croissant", "Panadería"},
	{"sprout", "Vegetariano"},
	{"popcorn", "Snacks"},
	{"cake-slice", "Postres"},
	{"ice-cream-cone", "Helados"},
	{"dessert", "Dulces"},
	{"donut", "Donas"},
	{"cookie", "Galletas"},
	{"cherry", "Frutas"},
	{"coffee", "Café"},
	{"milk", "Lácteos"},
	{"cup-soda", "Bebidas"},
	{"citrus", "Jugos"},
	{"glass-water", "Agua"},
	{"beer", "Cervezas"},
	{"wine", "Vinos"},
	{"martini", "Cócteles"},
}

var colores = []string{"blue", "green", "orange", "red", "yellow", "indigo", "purple", "teal", "pink", "gray"}

// maxPrecio evita errores de tipeo (un cero de más) en un restaurante.
var maxPrecio = decimal.NewFromInt(10_000)

type Service struct {
	DB *db.DB
	// ImagenURL convierte una clave de almacenamiento en una URL que el navegador puede cargar.
	ImagenURL func(key string, tam string) string
	Clock     interface{ Now() time.Time }
}

// ---------- Tarifas de IVA ----------

type TarifaIVA struct {
	ID          ids.ID `json:"id"`
	CodigoSRI   string `json:"codigoSri"`
	Porcentaje  string `json:"porcentaje"`
	Descripcion string `json:"descripcion"`
}

// TarifasVigentes devuelve las tarifas aplicables hoy (el IVA cambia por ley: RF-05-06).
func (s *Service) TarifasVigentes(ctx context.Context) ([]TarifaIVA, error) {
	var out []TarifaIVA
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, codigo_sri, porcentaje::text, descripcion FROM tarifas_iva
			WHERE vigente_desde <= $1 AND (vigente_hasta IS NULL OR vigente_hasta >= $1) ORDER BY tarifas_iva.porcentaje DESC, codigo_sri`, s.Clock.Now())
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[TarifaIVA])
		return err
	})
	return out, err
}

// ---------- Categorías ----------

type Categoria struct {
	ID         ids.ID   `json:"id"`
	Nombre     string   `json:"nombre"`
	Orden      int      `json:"orden"`
	EstacionID *ids.ID  `json:"estacionId"`
	Color      string   `json:"color"`
	Icono      string   `json:"icono"`
	Activa     bool     `json:"activa"`
	Productos  int      `json:"productos"`
	Notas      []string `json:"notasRapidas"`
}

type CategoriaInput struct {
	ID         *ids.ID  `json:"id"`
	Nombre     string   `json:"nombre"`
	EstacionID *ids.ID  `json:"estacionId"`
	Color      string   `json:"color"`
	Icono      string   `json:"icono"`
	Activa     *bool    `json:"activa"`
	Notas      []string `json:"notasRapidas"`
}

func (in *CategoriaInput) validar() error {
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Color == "" {
		in.Color = "orange"
	}
	if in.Icono == "" {
		in.Icono = "utensils"
	}
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Ponle un nombre de 1 a 40 caracteres, como «Ceviches» o «Bebidas».")
	v.Check(slices.Contains(colores, in.Color), "color", "Elige uno de los colores disponibles.")
	v.Check(slices.ContainsFunc(IconosCategoria, func(i IconoCategoria) bool { return i.Icono == in.Icono }), "icono", "Elige uno de los iconos de comida disponibles.")
	v.Check(len(in.Notas) <= 12, "notasRapidas", "Usa como máximo 12 notas rápidas por categoría.")
	for i, n := range in.Notas {
		in.Notas[i] = strings.TrimSpace(n)
		v.Check(len([]rune(in.Notas[i])) >= 1 && len([]rune(in.Notas[i])) <= 30, "notasRapidas", "Cada nota rápida va de 1 a 30 caracteres, como «Sin sal».")
	}
	return v.Err()
}

func (s *Service) Categorias(ctx context.Context, p auth.Principal) ([]Categoria, error) {
	var out []Categoria
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT c.id, c.nombre, c.orden, c.estacion_id, c.color, c.icono, c.activa,
			(SELECT count(*) FROM productos pr WHERE pr.categoria_id = c.id AND pr.deleted_at IS NULL)::int,
			coalesce((SELECT array_agg(n.texto ORDER BY n.orden) FROM notas_rapidas n WHERE n.categoria_id = c.id), '{}')
			FROM categorias c WHERE c.deleted_at IS NULL ORDER BY c.orden, c.nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Categoria])
		return err
	})
	return out, err
}

func (s *Service) CrearCategoria(ctx context.Context, p auth.Principal, in CategoriaInput) (Categoria, error) {
	if err := in.validar(); err != nil {
		return Categoria{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO categorias (id, tenant_id, nombre, orden, estacion_id, color, icono, created_by)
			VALUES ($1,$2,$3,(SELECT coalesce(max(orden),0)+1 FROM categorias WHERE deleted_at IS NULL),$4,$5,$6,$7) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.Nombre, in.EstacionID, in.Color, in.Icono, p.UserID)
		if err != nil {
			return db.Translate(err, "categorias_nombre", "nombre", "Ya tienes una categoría con ese nombre.")
		}
		if tag.RowsAffected() == 0 {
			return nil // reintento idempotente: ya existía con ese id
		}
		return guardarNotas(ctx, tx, p.TenantID, id, in.Notas)
	})
	if err != nil {
		return Categoria{}, err
	}
	return s.categoria(ctx, p, id)
}

func (s *Service) ActualizarCategoria(ctx context.Context, p auth.Principal, id ids.ID, in CategoriaInput) (Categoria, error) {
	if err := in.validar(); err != nil {
		return Categoria{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE categorias SET nombre=$2, estacion_id=$3, color=$4, icono=$5, activa=coalesce($6, activa) WHERE id=$1 AND deleted_at IS NULL`,
			id, in.Nombre, in.EstacionID, in.Color, in.Icono, in.Activa)
		if err != nil {
			return db.Translate(err, "categorias_nombre", "nombre", "Ya tienes una categoría con ese nombre.")
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		if in.Notas == nil {
			return nil
		}
		return guardarNotas(ctx, tx, p.TenantID, id, in.Notas)
	})
	if err != nil {
		return Categoria{}, err
	}
	return s.categoria(ctx, p, id)
}

func guardarNotas(ctx context.Context, tx db.Tx, tenant, categoria ids.ID, notas []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM notas_rapidas WHERE categoria_id=$1`, categoria); err != nil {
		return err
	}
	for i, n := range notas {
		if _, err := tx.Exec(ctx, `INSERT INTO notas_rapidas (id, tenant_id, categoria_id, texto, orden) VALUES ($1,$2,$3,$4,$5)`, ids.New(), tenant, categoria, n, i); err != nil {
			return err
		}
	}
	return nil
}

// OrdenarCategorias guarda el orden que el dueño eligió arrastrando.
func (s *Service) OrdenarCategorias(ctx context.Context, p auth.Principal, orden []ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		for i, id := range orden {
			if _, err := tx.Exec(ctx, `UPDATE categorias SET orden=$2 WHERE id=$1 AND orden <> $2`, id, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) EliminarCategoria(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM productos WHERE categoria_id=$1 AND deleted_at IS NULL`, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return apperr.New(apperr.Conflict, "CATEGORIA_CON_PRODUCTOS", fmt.Sprintf("Esta categoría tiene %d productos. Muévelos a otra categoría antes de eliminarla.", n))
		}
		tag, err := tx.Exec(ctx, `UPDATE categorias SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) categoria(ctx context.Context, p auth.Principal, id ids.ID) (Categoria, error) {
	cs, err := s.Categorias(ctx, p)
	for _, c := range cs {
		if c.ID == id {
			return c, err
		}
	}
	if err == nil {
		err = apperr.ErrNotFound
	}
	return Categoria{}, err
}

// ---------- Productos ----------

// Desglose muestra al dueño cómo se descompone el precio (RF-05-06.2).
type Desglose struct {
	Base  string `json:"base"`
	IVA   string `json:"iva"`
	Total string `json:"total"`
}

type Producto struct {
	ID                  ids.ID   `json:"id"`
	CategoriaID         ids.ID   `json:"categoriaId"`
	Nombre              string   `json:"nombre"`
	Alias               *string  `json:"alias"`
	Codigo              *string  `json:"codigo"`
	Descripcion         string   `json:"descripcion"`
	Precio              string   `json:"precio"`
	TarifaIVAID         ids.ID   `json:"tarifaIvaId"`
	Tipo                string   `json:"tipo"`
	ComportamientoStock string   `json:"comportamientoStock"`
	EstacionID          *ids.ID  `json:"estacionId"`
	ImagenKey           *string  `json:"imagenKey"`
	ImagenURL           *string  `json:"imagenUrl"`
	ImagenMiniURL       *string  `json:"imagenMiniUrl"`
	Activo              bool     `json:"activo"`
	VisibleMenuQR       bool     `json:"visibleMenuQr"`
	Orden               int      `json:"orden"`
	Grupos              []ids.ID `json:"gruposModificadores"`
	Desglose            Desglose `json:"desglose"`
}

type ProductoInput struct {
	ID                  *ids.ID  `json:"id"`
	CategoriaID         ids.ID   `json:"categoriaId"`
	Nombre              string   `json:"nombre"`
	Alias               *string  `json:"alias"`
	Codigo              *string  `json:"codigo"`
	Descripcion         string   `json:"descripcion"`
	Precio              string   `json:"precio"`
	TarifaIVAID         ids.ID   `json:"tarifaIvaId"`
	Tipo                string   `json:"tipo"`
	ComportamientoStock string   `json:"comportamientoStock"`
	EstacionID          *ids.ID  `json:"estacionId"`
	ImagenKey           *string  `json:"imagenKey"`
	Activo              *bool    `json:"activo"`
	VisibleMenuQR       *bool    `json:"visibleMenuQr"`
	Grupos              []ids.ID `json:"gruposModificadores"`
}

func (in *ProductoInput) validar() error {
	in.Nombre = strings.TrimSpace(in.Nombre)
	in.Descripcion = strings.TrimSpace(in.Descripcion)
	in.Alias = trimOrNil(in.Alias, true)
	in.Codigo = trimOrNil(in.Codigo, false)
	if in.Tipo == "" {
		in.Tipo = "SIMPLE"
	}
	if in.ComportamientoStock == "" {
		in.ComportamientoStock = "NINGUNO"
	}
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 80, "nombre", "Ponle un nombre de 1 a 80 caracteres, como «Ceviche mixto».")
	v.Check(in.CategoriaID != ids.Nil, "categoriaId", "Elige la categoría del producto.")
	v.Check(in.TarifaIVAID != ids.Nil, "tarifaIvaId", "Elige la tarifa de IVA (normalmente 15 %).")
	v.Check(len([]rune(in.Descripcion)) <= 300, "descripcion", "La descripción admite hasta 300 caracteres.")
	if in.Alias != nil {
		v.Check(len(*in.Alias) <= 12 && !strings.Contains(*in.Alias, " "), "alias", "El código rápido va sin espacios y con hasta 12 caracteres, como «CM».")
	}
	if in.Codigo != nil {
		v.Check(len(*in.Codigo) <= 25, "codigo", "El código admite hasta 25 caracteres.")
	}
	d, err := decimal.NewFromString(strings.TrimSpace(in.Precio))
	switch {
	case err != nil:
		v.Add("precio", "Escribe el precio con punto decimal, por ejemplo 12.50.")
	case d.IsNegative():
		v.Add("precio", "El precio no puede ser negativo.")
	case d.GreaterThan(maxPrecio):
		v.Add("precio", "El precio parece demasiado alto. Revisa que no sobre un cero.")
	case d.Exponent() < -int32(money.UnitPriceScale):
		v.Add("precio", "Usa como máximo 6 decimales.")
	default:
		in.Precio = d.String()
	}
	v.Check(slices.Contains([]string{"SIMPLE", "RECETA", "PREPARACION", "INSUMO_VENDIBLE"}, in.Tipo), "tipo", "Tipo de producto desconocido.")
	v.Check(slices.Contains([]string{"NINGUNO", "PERMANENTE", "DIARIO"}, in.ComportamientoStock), "comportamientoStock", "Control de stock desconocido.")
	v.Check(len(in.Grupos) <= 10, "gruposModificadores", "Usa como máximo 10 grupos de modificadores por producto.")
	return v.Err()
}

func trimOrNil(s *string, upper bool) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	if upper {
		t = strings.ToUpper(t)
	}
	return &t
}

type Filtro struct {
	CategoriaID *ids.ID
	Buscar      string
}

const productoCols = `p.id, p.categoria_id, p.nombre, p.alias, p.codigo, p.descripcion, p.precio::text, p.tarifa_iva_id, p.tipo,
	p.comportamiento_stock, p.estacion_id, p.imagen_key, p.activo, p.visible_menu_qr, p.orden,
	coalesce((SELECT array_agg(g.grupo_id ORDER BY g.orden) FROM producto_grupos_modificadores g WHERE g.producto_id = p.id), '{}'),
	t.porcentaje::text, l.precios_incluyen_iva`

func (s *Service) Productos(ctx context.Context, p auth.Principal, f Filtro) ([]Producto, error) {
	var out []Producto
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		q := `SELECT ` + productoCols + ` FROM productos p JOIN tarifas_iva t ON t.id = p.tarifa_iva_id
			CROSS JOIN LATERAL (SELECT precios_incluyen_iva FROM locales WHERE deleted_at IS NULL ORDER BY codigo_establecimiento LIMIT 1) l
			WHERE p.deleted_at IS NULL AND ($1::uuid IS NULL OR p.categoria_id = $1)
			  AND ($2 = '' OR p.nombre ILIKE '%' || $2 || '%' OR upper(p.alias) = upper($2) OR p.codigo = $2)
			ORDER BY p.orden, p.nombre`
		rows, err := tx.Query(ctx, q, f.CategoriaID, strings.TrimSpace(f.Buscar))
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, s.scanProducto)
		return err
	})
	return out, err
}

func (s *Service) Producto(ctx context.Context, p auth.Principal, id ids.ID) (Producto, error) {
	var out Producto
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+productoCols+` FROM productos p JOIN tarifas_iva t ON t.id = p.tarifa_iva_id
			CROSS JOIN LATERAL (SELECT precios_incluyen_iva FROM locales WHERE deleted_at IS NULL ORDER BY codigo_establecimiento LIMIT 1) l
			WHERE p.id = $1 AND p.deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		out, err = pgx.CollectExactlyOneRow(rows, s.scanProducto)
		return db.NotFound(err)
	})
	return out, err
}

func (s *Service) scanProducto(r pgx.CollectableRow) (Producto, error) {
	var pr Producto
	var porcentaje string
	var incluye bool
	err := r.Scan(&pr.ID, &pr.CategoriaID, &pr.Nombre, &pr.Alias, &pr.Codigo, &pr.Descripcion, &pr.Precio, &pr.TarifaIVAID, &pr.Tipo,
		&pr.ComportamientoStock, &pr.EstacionID, &pr.ImagenKey, &pr.Activo, &pr.VisibleMenuQR, &pr.Orden, &pr.Grupos, &porcentaje, &incluye)
	if err != nil {
		return pr, err
	}
	pr.Precio = decimal.RequireFromString(pr.Precio).String()
	pr.Desglose = Desglosar(pr.Precio, porcentaje, incluye)
	if pr.ImagenKey != nil && s.ImagenURL != nil {
		u, m := s.ImagenURL(*pr.ImagenKey, "md"), s.ImagenURL(*pr.ImagenKey, "sm")
		pr.ImagenURL, pr.ImagenMiniURL = &u, &m
	}
	return pr, nil
}

// Desglosar calcula base, IVA y total de un precio unitario (docs/05 §8, sin redondeo
// por centavo de líneas: es informativo para el dueño; el cálculo fiscal está en F5-04).
func Desglosar(precio, porcentaje string, incluyeIVA bool) Desglose {
	p := decimal.RequireFromString(precio)
	t := decimal.RequireFromString(porcentaje).Div(decimal.NewFromInt(100))
	var base, total decimal.Decimal
	if incluyeIVA {
		total = p
		base = p.Div(decimal.NewFromInt(1).Add(t)).Round(6)
	} else {
		base = p
		total = p.Add(p.Mul(t))
	}
	b, tot := money.FromDecimal(base).Round2(), money.FromDecimal(total).Round2()
	return Desglose{Base: b.String(), IVA: tot.Sub(b).String(), Total: tot.String()}
}

func (s *Service) CrearProducto(ctx context.Context, p auth.Principal, in ProductoInput) (Producto, error) {
	if err := in.validar(); err != nil {
		return Producto{}, err
	}
	if in.ImagenKey != nil && !imagenes.ClaveValida(p.TenantID, *in.ImagenKey) {
		return Producto{}, &apperr.Error{
			Kind: apperr.Invalid, Code: "IMAGEN_INVALIDA", Message: "Esa foto no existe. Súbela de nuevo o elige una de la galería.",
			Fields: []apperr.FieldError{{Campo: "imagenKey", Mensaje: "Esa foto no existe."}},
		}
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if err := s.tarifaVigente(ctx, tx, in.TarifaIVAID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO productos (id, tenant_id, categoria_id, nombre, alias, codigo, descripcion, precio, tarifa_iva_id, tipo,
			comportamiento_stock, estacion_id, imagen_key, activo, visible_menu_qr, orden, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,coalesce($14,true),coalesce($15,true),
			  (SELECT coalesce(max(orden),0)+1 FROM productos WHERE categoria_id=$3 AND deleted_at IS NULL),$16)
			ON CONFLICT (id) DO NOTHING`, id, p.TenantID, in.CategoriaID, in.Nombre, in.Alias, in.Codigo, in.Descripcion, in.Precio, in.TarifaIVAID,
			in.Tipo, in.ComportamientoStock, in.EstacionID, in.ImagenKey, in.Activo, in.VisibleMenuQR, p.UserID)
		if err != nil {
			return traducirProducto(err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return guardarGrupos(ctx, tx, p.TenantID, id, in.Grupos)
	})
	if err != nil {
		return Producto{}, err
	}
	return s.Producto(ctx, p, id)
}

func (s *Service) ActualizarProducto(ctx context.Context, p auth.Principal, id ids.ID, in ProductoInput) (Producto, error) {
	if err := in.validar(); err != nil {
		return Producto{}, err
	}
	if in.ImagenKey != nil && !imagenes.ClaveValida(p.TenantID, *in.ImagenKey) {
		return Producto{}, &apperr.Error{
			Kind: apperr.Invalid, Code: "IMAGEN_INVALIDA", Message: "Esa foto no existe. Súbela de nuevo o elige una de la galería.",
			Fields: []apperr.FieldError{{Campo: "imagenKey", Mensaje: "Esa foto no existe."}},
		}
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if err := s.tarifaVigente(ctx, tx, in.TarifaIVAID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE productos SET categoria_id=$2, nombre=$3, alias=$4, codigo=$5, descripcion=$6, precio=$7, tarifa_iva_id=$8,
			tipo=$9, comportamiento_stock=$10, estacion_id=$11, imagen_key=$12, activo=coalesce($13, activo), visible_menu_qr=coalesce($14, visible_menu_qr)
			WHERE id=$1 AND deleted_at IS NULL`, id, in.CategoriaID, in.Nombre, in.Alias, in.Codigo, in.Descripcion, in.Precio, in.TarifaIVAID,
			in.Tipo, in.ComportamientoStock, in.EstacionID, in.ImagenKey, in.Activo, in.VisibleMenuQR)
		if err != nil {
			return traducirProducto(err)
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return guardarGrupos(ctx, tx, p.TenantID, id, in.Grupos)
	})
	if err != nil {
		return Producto{}, err
	}
	return s.Producto(ctx, p, id)
}

func (s *Service) EliminarProducto(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		// Borrado lógico: las ventas pasadas guardan su copia (snapshot) del nombre y precio.
		tag, err := tx.Exec(ctx, `UPDATE productos SET deleted_at=now(), alias=NULL, codigo=NULL WHERE id=$1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) tarifaVigente(ctx context.Context, tx db.Tx, id ids.ID) error {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tarifas_iva WHERE id=$1 AND vigente_desde <= $2 AND (vigente_hasta IS NULL OR vigente_hasta >= $2))`, id, s.Clock.Now()).Scan(&ok)
	if err == nil && !ok {
		return &apperr.Error{
			Kind: apperr.Invalid, Code: "TARIFA_NO_VIGENTE", Message: "Esa tarifa de IVA ya no está vigente. Elige otra.",
			Fields: []apperr.FieldError{{Campo: "tarifaIvaId", Mensaje: "Esa tarifa de IVA ya no está vigente."}},
		}
	}
	return err
}

func guardarGrupos(ctx context.Context, tx db.Tx, tenant, producto ids.ID, grupos []ids.ID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM producto_grupos_modificadores WHERE producto_id=$1`, producto); err != nil {
		return err
	}
	for i, g := range grupos {
		if _, err := tx.Exec(ctx, `INSERT INTO producto_grupos_modificadores (tenant_id, producto_id, grupo_id, orden) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			tenant, producto, g, i); err != nil {
			return db.Translate(err, "", "", "")
		}
	}
	return nil
}

func traducirProducto(err error) error {
	switch {
	case db.IsUniqueViolation(err, "productos_alias"):
		return db.Translate(err, "productos_alias", "alias", "Ese código rápido ya lo usa otro producto.")
	case db.IsUniqueViolation(err, "productos_codigo"):
		return db.Translate(err, "productos_codigo", "codigo", "Ese código ya lo usa otro producto.")
	}
	return db.Translate(err, "", "", "")
}
