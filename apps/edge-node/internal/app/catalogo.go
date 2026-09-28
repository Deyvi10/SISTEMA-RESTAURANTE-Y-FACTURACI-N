package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Catálogo que baja el teléfono (F3-05): todo lo necesario para tomar pedidos sin red.

type CategoriaApp struct {
	ID     ids.ID   `json:"id"`
	Nombre string   `json:"nombre"`
	Icono  string   `json:"icono"`
	Color  string   `json:"color"`
	Orden  int      `json:"orden"`
	Notas  []string `json:"notasRapidas"`
}

type ModificadorApp struct {
	ID              ids.ID `json:"id"`
	Nombre          string `json:"nombre"`
	PrecioAdicional string `json:"precioAdicional"`
}

type GrupoApp struct {
	ID            ids.ID           `json:"id"`
	Nombre        string           `json:"nombre"`
	Obligatorio   bool             `json:"obligatorio"`
	Min           int              `json:"min"`
	Max           int              `json:"max"`
	Modificadores []ModificadorApp `json:"modificadores"`
}

type ProductoApp struct {
	ID          ids.ID   `json:"id"`
	CategoriaID ids.ID   `json:"categoriaId"`
	Nombre      string   `json:"nombre"`
	Alias       string   `json:"alias"`
	Descripcion string   `json:"descripcion"`
	Precio      string   `json:"precio"`
	Foto        *string  `json:"foto"`   // /media/<clave>/sm.webp (servida por el nodo)
	FotoMd      *string  `json:"fotoMd"` // /media/<clave>/md.webp
	Grupos      []ids.ID `json:"grupos"`
	Vendidos    int      `json:"vendidos"` // últimos 30 días: ordena la búsqueda
	Orden       int      `json:"orden"`
}

type CatalogoApp struct {
	Version    string         `json:"version"` // cambia cuando cambia algo
	Categorias []CategoriaApp `json:"categorias"`
	Productos  []ProductoApp  `json:"productos"`
	Grupos     []GrupoApp     `json:"grupos"`
}

func (a *App) Catalogo(ctx context.Context) (CatalogoApp, error) {
	q := a.Store.Read()
	c := CatalogoApp{Categorias: []CategoriaApp{}, Productos: []ProductoApp{}, Grupos: []GrupoApp{}}
	var cursor sql.NullInt64
	_ = q.QueryRowContext(ctx, `SELECT cursor FROM inbox_cursores WHERE flujo = ?`, flujoNube).Scan(&cursor)
	c.Version = itoa(int(cursor.Int64))

	rows, err := q.QueryContext(ctx, `SELECT id, nombre, coalesce(icono, 'utensils'), coalesce(color, 'orange'), orden FROM categorias
		WHERE deleted_at IS NULL AND activa = 1 ORDER BY orden, nombre`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var x CategoriaApp
		var id string
		if err := rows.Scan(&id, &x.Nombre, &x.Icono, &x.Color, &x.Orden); err != nil {
			_ = rows.Close()
			return c, err
		}
		x.ID, _ = ids.Parse(id)
		x.Notas = []string{}
		c.Categorias = append(c.Categorias, x)
	}
	_ = rows.Close()
	for i := range c.Categorias {
		nr, err := q.QueryContext(ctx, `SELECT texto FROM notas_rapidas WHERE categoria_id = ? ORDER BY orden`, c.Categorias[i].ID.String())
		if err != nil {
			return c, err
		}
		for nr.Next() {
			var t string
			if nr.Scan(&t) == nil {
				c.Categorias[i].Notas = append(c.Categorias[i].Notas, t)
			}
		}
		_ = nr.Close()
	}

	desde := a.Clock.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	rows, err = q.QueryContext(ctx, `SELECT p.id, p.categoria_id, p.nombre, coalesce(p.alias, ''), p.descripcion, p.precio, p.imagen_key, p.orden,
			(SELECT count(*) FROM orden_lineas l WHERE l.producto_id = p.id AND l.estado <> 'ANULADA' AND l.creada_at >= ?)
		FROM productos p JOIN categorias c ON c.id = p.categoria_id AND c.deleted_at IS NULL AND c.activa = 1
		WHERE p.deleted_at IS NULL AND p.activo = 1 ORDER BY c.orden, p.orden, p.nombre`, desde)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var x ProductoApp
		var id, cat string
		var img sql.NullString
		if err := rows.Scan(&id, &cat, &x.Nombre, &x.Alias, &x.Descripcion, &x.Precio, &img, &x.Orden, &x.Vendidos); err != nil {
			_ = rows.Close()
			return c, err
		}
		x.ID, _ = ids.Parse(id)
		x.CategoriaID, _ = ids.Parse(cat)
		if img.Valid && img.String != "" {
			sm, md := "/media/"+img.String+"/sm.webp", "/media/"+img.String+"/md.webp"
			x.Foto, x.FotoMd = &sm, &md
		}
		x.Grupos = []ids.ID{}
		c.Productos = append(c.Productos, x)
	}
	_ = rows.Close()
	porProd := map[ids.ID]int{}
	for i, p := range c.Productos {
		porProd[p.ID] = i
	}
	pg, err := q.QueryContext(ctx, `SELECT producto_id, grupo_id FROM producto_grupos_modificadores ORDER BY orden`)
	if err != nil {
		return c, err
	}
	for pg.Next() {
		var p, g string
		if pg.Scan(&p, &g) == nil {
			pid, _ := ids.Parse(p)
			gid, _ := ids.Parse(g)
			if i, ok := porProd[pid]; ok {
				c.Productos[i].Grupos = append(c.Productos[i].Grupos, gid)
			}
		}
	}
	_ = pg.Close()

	rows, err = q.QueryContext(ctx, `SELECT id, nombre, obligatorio, min, max FROM grupos_modificadores WHERE deleted_at IS NULL ORDER BY nombre`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var g GrupoApp
		var id string
		var ob int
		if err := rows.Scan(&id, &g.Nombre, &ob, &g.Min, &g.Max); err != nil {
			_ = rows.Close()
			return c, err
		}
		g.ID, _ = ids.Parse(id)
		g.Obligatorio = ob == 1
		g.Modificadores = []ModificadorApp{}
		c.Grupos = append(c.Grupos, g)
	}
	_ = rows.Close()
	for i := range c.Grupos {
		mr, err := q.QueryContext(ctx, `SELECT id, nombre, precio_adicional FROM modificadores WHERE grupo_id = ? AND activo = 1 ORDER BY orden, nombre`, c.Grupos[i].ID.String())
		if err != nil {
			return c, err
		}
		for mr.Next() {
			var m ModificadorApp
			var id string
			if mr.Scan(&id, &m.Nombre, &m.PrecioAdicional) == nil {
				m.ID, _ = ids.Parse(id)
				c.Grupos[i].Modificadores = append(c.Grupos[i].Modificadores, m)
			}
		}
		_ = mr.Close()
	}
	return c, nil
}
