# 09 · Modelo de negocio y salida al mercado

## 1. Planes (propuesta inicial, precios en USD + IVA)

| | **Emprendedor** | **Restaurante** ⭐ | **Pro / Cadena** |
|---|---|---|---|
| Precio orientativo | $29-35 / mes | $59-75 / mes | $110+ / mes por local |
| Público | Cafeterías, food trucks, huecas | Restaurantes medianos, bares, pizzerías | Marcas con 2+ locales |
| Usuarios caja | 1 | Ilimitados | Ilimitados |
| Meseros / dispositivos | 1 / 2 | Ilimitados* | Ilimitados* |
| Impresoras / estaciones | 1 | Múltiples | Múltiples |
| Facturación electrónica | Ilimitada | Ilimitada | Ilimitada |
| Nodo Local (offline) | ✅ | ✅ | ✅ (+ standby, futuro) |
| Inventario | Productos simples | Recetas, sub-recetas, mermas, stock diario | + traslados entre locales |
| División de cuentas | Partes iguales | Completa | Completa |
| KDS / Menú QR | — / ✅ básico | ✅ / ✅ | ✅ / ✅ |
| Reportes | Básicos | Completos + propinas | + consolidado multi-local, auditoría avanzada |
| 2FA obligatorio | — | Opcional | ✅ |

\* Sujeto a uso razonable. Todos los límites se configuran en datos (RF-09-01).

**Costos a considerar en el precio:** infraestructura por tenant, almacenamiento de comprobantes durante 7 años, correo transaccional, comisiones de la pasarela, soporte e IVA. Definir el **costo unitario por tenant** en la Fase 6 con datos reales del piloto.

## 2. Perfil de cliente ideal (ICP) para los primeros 10

- Restaurantes independientes de alto tráfico (hamburgueserías, pizzerías, cevicherías) con 5 a 15 mesas, 2 a 4 meseros y 1 cajero.
- Dolor actual: sistema web que se cae los fines de semana, facturación a mano cuando cae el SRI, gritos a la cocina.
- Zona geográfica concentrada (clúster) para aprovechar el boca a boca.

## 3. Estrategia de adquisición

1. **Demo "anti-caídas" en el local:** kit con laptop (Nodo Local), celular, router y térmica. Se toma un pedido, se imprime en menos de 1 s, **se desconecta el internet** y se sigue operando.
2. **Oferta de riesgo cero para early adopters:** sin costo de implementación, migración del menú incluida, **14 días en paralelo** con su sistema actual y cláusula de éxito (si funciona, contratan y graban un testimonio de 30 s).
3. **Expansión por clúster:** usar el primer cliente como referencia con los locales vecinos.
4. **Objeciones:**
   - "Ya tengo un sistema contable" → es un POS, no un ERP; exporta a Excel para el contador.
   - "Me da miedo perder mi menú" → migración asistida.

## 4. Cobro de suscripciones

- Pasarela que opere con comercios domiciliados en Ecuador (Stripe no está disponible para ellos): **Payphone, Kushki, Datafast, PlacetoPay** u otra. Hay que evaluar la tokenización y el cobro recurrente, las comisiones y la API → **DP-06**.
- Alternativa inicial (piloto y primeros 10): cobro por transferencia o débito manual + factura electrónica emitida por el propio sistema.

## 5. Legal y soporte

- Documentos a preparar antes de la Fase 10: términos y condiciones, política de privacidad, acuerdo de tratamiento de datos (LOPDP), SLA.
- SLA propuesto: soporte por WhatsApp y correo en horario extendido de restaurante (incluido fines de semana), con tiempo de primera respuesta ≤ 30 min para S1.
- Estrategia de retención: el Nodo Local (operación sin internet) es el principal ancla de valor.
