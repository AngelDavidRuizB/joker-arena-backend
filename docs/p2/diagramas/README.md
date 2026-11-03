# Diagramas del Prototipo 2

Cada vista tiene su fuente editable (`.mmd`, Mermaid) y la imagen que usa `p2_2b.md` (`.png`).
Si editas un `.mmd`, vuelve a generar su `.png` para que el PDF quede actualizado.

| Vista | Fuente |
|---|---|
| Componentes y Conectores (C&C) | `cc_view_p2.mmd` |
| Despliegue | `despliegue_view_p2.mmd` |
| Capas | `capas_view_p2.mmd` |
| Descomposición | `descomposicion_view_p2.mmd` |

## Generar un PNG

Opción rápida: pegar el `.mmd` en https://mermaid.live y exportar como PNG.

Opción por terminal (necesita Node.js):

```bash
npx -y @mermaid-js/mermaid-cli@11.4.2 -i cc_view_p2.mmd -o cc_view_p2.png -b white -s 2
```
