-- F5-05 · RUC del proveedor del sistema de facturación: obligatorio en cada comprobante
-- emitido con un sistema de terceros, como campo adicional «RUC Proveedor» (Anexo 26 de la
-- ficha técnica offline v2.34, Res. NAC-DGERCGC26-00000027). Global: es el RUC de quien
-- provee este software, no del restaurante. Vacío hasta que se defina.

-- +goose Up
INSERT INTO parametros_globales (clave, valor, descripcion) VALUES
  ('ruc_proveedor_sistema', '', 'RUC del proveedor del sistema de facturación electrónica (Anexo 26 de la ficha del SRI).')
ON CONFLICT (clave) DO NOTHING;

-- +goose Down
DELETE FROM parametros_globales WHERE clave = 'ruc_proveedor_sistema';
