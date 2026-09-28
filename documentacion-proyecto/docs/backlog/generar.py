"""Genera docs/12-backlog-tickets.md a partir de fases.json y tickets.json.

Uso: python3 docs/backlog/generar.py
Los JSON son la fuente de verdad del backlog; no edites el .md a mano.
"""
import json
from pathlib import Path

AQUI = Path(__file__).parent
fases = json.loads((AQUI / "fases.json").read_text(encoding="utf-8"))
tickets = json.loads((AQUI / "tickets.json").read_text(encoding="utf-8"))

PRIO = {"M": "Must", "S": "Should", "C": "Could", "W": "Won't (por ahora)"}


def semanas(r):
    return "—" if r == [0, 0] else f"{r[0]}-{r[1]} sem"


out = [
    "# 12 · Backlog por fases (tickets)",
    "",
    "> Generado por `docs/backlog/generar.py` desde `docs/backlog/*.json`. **No editar a mano**: cambia el JSON y vuelve a generar.",
    "> Cada ticket traza a sus requisitos (`RF-*`, `RNF-*`, hallazgos de la revisión, ADR). Prioridad MoSCoW; talla orientativa S/M/L/XL.",
    "",
    "## Resumen",
    "",
    "| Fase | Nombre | Tickets | 1 dev | Equipo de 3 | MVP |",
    "|---|---|---|---|---|---|",
]
for f in fases:
    n = sum(1 for t in tickets if t["id"].split("-")[0] == f["id"])
    out.append(
        f"| {f['id']} | {f['nombre']} | {n} | {semanas(f['sem1'])} | {semanas(f['sem3'])} | {'✅' if f['mvp'] else ''} |"
    )
out += [
    "",
    f"**Total:** {len(tickets)} tickets. Las semanas de F0-F6 vienen de `07` §4; las de F7-F10 reparten entre fases el total que da ese mismo documento (+6-9 meses con 1 dev, +4-6 meses con 3).",
    "",
]

for f in fases:
    out += [f"## {f['id']} · {f['nombre']}", "", f"**Objetivo:** {f['objetivo']}", ""]
    out += [f"**Requisitos:** {f['reqs']}", "", "**Definition of Done de la fase:**", ""]
    out += [f"- [ ] {d}" for d in f["dod"]]
    out.append("")
    for t in (t for t in tickets if t["id"].split("-")[0] == f["id"]):
        out += [f"### {t['id']} · {t['t']}", ""]
        meta = [
            f"**Tipo:** {t['tipo']}",
            f"**Prioridad:** {PRIO[t['prio']]}",
            f"**Talla:** {t['talla']}",
            f"**Área:** {', '.join('`' + a + '`' for a in t['area'])}",
        ]
        out += [" · ".join(meta), ""]
        if t["req"]:
            out += [f"**Trazabilidad:** {', '.join(t['req'])}", ""]
        out += [f"**Qué se quiere:** {t['que']}", "", "**Criterios de aceptación:**", ""]
        out += [f"- [ ] {a}" for a in t["ac"]]
        out.append("")
        if t.get("tec"):
            out += ["**Notas técnicas:**", ""] + [f"- {x}" for x in t["tec"]] + [""]
        if t.get("ui"):
            out += [f"**Diseño (estilo iOS):** {t['ui']}", ""]
        if t["dep"]:
            out += [f"**Depende de:** {', '.join(t['dep'])}", ""]

(AQUI.parent / "12-backlog-tickets.md").write_text("\n".join(out), encoding="utf-8")
print(f"OK: {len(tickets)} tickets en {len(fases)} fases")
