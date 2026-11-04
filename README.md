# Peru SDK para Go

SDK nativo Go 1.26+, licencia MIT, sin dependencias de runtime de terceros. Incluye un cliente HTTP real de Consulta Integrada SUNAT y un proveedor de identidad exclusivamente sintético. La compatibilidad institucional con credenciales reales sigue pendiente de verificación.

## Uso

Una vez fusionada la implementación, se podrá usar `go get github.com/DataScienceResearchPeru/peru-sdk-go@main`; para reproducibilidad, fijar un commit o una versión publicada. El módulo está en la raíz: las futuras etiquetas son `v0.1.0-alpha.1`, sin prefijo de subdirectorio.

```go
import (
    "context"
    "github.com/DataScienceResearchPeru/peru-sdk-go/sunat"
)

func check(ctx context.Context, requesterRUC, accessToken string) (sunat.ValidityResult, error) {
    client, err := sunat.NewValidityClient(sunat.ValidityOptions{
        RequesterRUC: requesterRUC,
        TokenSource: sunat.StaticToken(accessToken),
    })
    if err != nil { return sunat.ValidityResult{}, err }
    return client.Check(ctx, sunat.ValidityQuery{
        IssuerRUC: "20000000001", DocumentType: "01", Series: "F001", Number: 123,
        IssuedOn: "2026-11-02", Amount: "118.00",
    })
}
```

La llamada sí realiza HTTP contra SUNAT. Los identificadores del ejemplo son sintéticos: sustituirlos por datos propios autorizados antes de hacer una consulta. OAuth se configura con `NewOAuth2TokenSource`: es `client_credentials`, no la Clave SOL de emisión.

Ejemplo sin red: `go run ./examples/offline`. Pruebas sin red externa: `go test -race ./...`; también funcionan con el repositorio clonado en cualquier carpeta, sin otros SDKs presentes. `CGO_ENABLED=0 go test ./...` comprueba la implementación nativa.

## Modelos y corpus

`upstream.lock.json` fija commits completos y hashes del generador, los esquemas y los 37 casos sintéticos. Los modelos y `internal/conformance/cases.json` se versionan en este repositorio. La sincronización explícita de desarrollo requiere Python y gofmt:

```bash
python3 .ci/sync_upstream.py --check
# Para actualizar desde las referencias revisadas del lock:
python3 .ci/sync_upstream.py
```

Instalar y usar el SDK no ejecuta el generador ni descarga esos repositorios. Cambiar un contrato común no publica una nueva versión Go automáticamente.

## Distribución y releases

CI prueba el módulo mínimo/actual en Linux y la versión actual sin cgo en Windows. El workflow Release construye un archivo fuente, lo extrae fuera del checkout, ejecuta su suite y compila un consumidor externo. Los artefactos incluyen versión, commit, referencias comunes y SHA-256.

Para una distribución local: `stage=$(mktemp -d /var/tmp/peru-go-dist.XXXXXXXX)` y `python3 .ci/check_package.py --output "$stage"` desde un checkout con sus archivos añadidos a Git.

La ejecución manual de Release tiene `publish=false` por defecto. Con `publish=true`, solo desde main y después de pasar las comprobaciones, crea la etiqueta de VERSION y la release de GitHub usando GITHUB_TOKEN. No sobrescribe etiquetas ni artefactos existentes. No necesita una cuenta en un registro Go. No se ha publicado aún una versión etiquetada.

## Límites

No implementa emisión UBL, firma ni CDR todavía. `reniec.MockIdentityService` no se conecta a RENIEC ni acredita identidades. Los estados desconocidos se conservan; una consulta HTTP exitosa no implica aceptación del comprobante. Los importes son decimales exactos representados públicamente como strings.
