// Teclado de PIN de 4 dígitos: con el mouse, la pantalla táctil o el teclado numérico.
import { Delete } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

const LETRAS: Record<string, string> = { "2": "ABC", "3": "DEF", "4": "GHI", "5": "JKL", "6": "MNO", "7": "PQRS", "8": "TUV", "9": "WXYZ" };

export function TecladoPIN({ alCompletar, largo = 4 }: { alCompletar: (pin: string) => Promise<string | null>; largo?: number }) {
  const [pin, setPin] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);

  const teclear = useCallback(
    (d: string) => {
      if (ocupado) return;
      setError(null);
      setPin((p) => {
        const n = (p + d).slice(0, largo);
        if (n.length === largo) {
          setOcupado(true);
          void alCompletar(n).then((err) => {
            setOcupado(false);
            if (err) {
              setError(err);
              setPin("");
            }
          });
        }
        return n;
      });
    },
    [alCompletar, largo, ocupado],
  );
  const borrar = useCallback(() => setPin((p) => p.slice(0, -1)), []);

  useEffect(() => {
    const alTeclear = (e: KeyboardEvent) => {
      if (/^\d$/.test(e.key)) teclear(e.key);
      else if (e.key === "Backspace") borrar();
    };
    window.addEventListener("keydown", alTeclear);
    return () => window.removeEventListener("keydown", alTeclear);
  }, [teclear, borrar]);

  return (
    <div className="teclado-pin">
      <div className="rp-pin-dots" data-error={error ? "true" : undefined} aria-label={`${pin.length} de ${largo} dígitos`}>
        {Array.from({ length: largo }, (_, i) => (
          <i key={i} data-filled={i < pin.length ? "true" : undefined} />
        ))}
      </div>
      <p className="teclado-pin__error" role="alert">
        {error ?? " "}
      </p>
      <div className="rp-keypad">
        {["1", "2", "3", "4", "5", "6", "7", "8", "9"].map((d) => (
          <button key={d} type="button" className="rp-key" onClick={() => teclear(d)} disabled={ocupado}>
            {d}
            {LETRAS[d] && <small>{LETRAS[d]}</small>}
          </button>
        ))}
        <span className="rp-key rp-key--blank" />
        <button type="button" className="rp-key" onClick={() => teclear("0")} disabled={ocupado}>
          0
        </button>
        <button type="button" className="rp-key rp-key--text" onClick={borrar} aria-label="Borrar">
          <Delete />
        </button>
      </div>
    </div>
  );
}
