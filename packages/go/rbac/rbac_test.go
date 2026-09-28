package rbac

import "testing"

// Casos tomados literalmente de la matriz de docs/requisitos/README.md §3.
func TestMatrizPorDefecto(t *testing.T) {
	casos := []struct {
		rol  Rol
		p    Permiso
		want bool
	}{
		{Admin, ConfigurarSRI, true},
		{Admin, VerEsperadoCierre, true},
		{Mesero, TomarPedido, true},
		{Mesero, Cobrar, false},
		{Mesero, AnularItemEnviado, false},
		{Cajero, Cobrar, true},
		{Cajero, DarDescuento, false},
		{Cajero, VerEsperadoCierre, false},
		{Cocina, MarcarListo, true},
		{Cocina, TomarPedido, false},
		{Mesero, ConfigurarMenu, false},
		{Cajero, GestionarPersonal, false},
	}
	for _, c := range casos {
		if got := Defecto(c.rol, c.p); got != c.want {
			t.Errorf("%s/%s = %v, se esperaba %v", c.rol, c.p, got, c.want)
		}
	}
}

func TestAjusteSoloSiEsConfigurable(t *testing.T) {
	ef := Efectivos(Mesero, map[Permiso]bool{Cobrar: true, ConfigurarSRI: true})
	if !ef[Cobrar] {
		t.Error("Cobrar es ⚙️ para Mesero: el ajuste debe aplicarse")
	}
	if ef[ConfigurarSRI] {
		t.Error("ConfigurarSRI no es configurable para Mesero: el ajuste debe ignorarse")
	}
	if Efectivos(Admin, map[Permiso]bool{ConfigurarMenu: false})[ConfigurarMenu] != true {
		t.Error("al Admin no se le quitan permisos por ajuste")
	}
}

func TestCatalogoCompleto(t *testing.T) {
	for _, i := range Catalogo() {
		if i.Nombre == "" || i.Descripcion == "" {
			t.Errorf("%s sin nombre o descripción para la UI", i.Permiso)
		}
		if !Defecto(Admin, i.Permiso) {
			t.Errorf("el Admin debe tener %s por defecto", i.Permiso)
		}
	}
}
