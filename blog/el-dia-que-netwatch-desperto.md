# El día que netwatch despertó

## Construyendo el pipeline

Con el diseño listo, llegó el momento de escribir código de verdad. La arquitectura ya estaba definida: cuatro fases en secuencia — ICMP, ARP, DNS y OUI — coordinadas por un orquestador. Cada fase recibe la entrada de la anterior y la enriquece.

La primera decisión importante fue cómo estructurar las interfaces. En lugar de un monolito `Scanner` que hiciera todo, opté por interfaces separadas: `Pinger`, `ARPReader`, `DNSResolver`, `OUILookup`. Cada una con una responsabilidad única y su propio conjunto de tests. El orquestador las recibe por inyección de dependencias y las ejecuta en orden.

```go
type Pinger interface {
    Ping(ctx context.Context, subnet string, concurrency int) ([]string, error)
}
type ARPReader interface {
    Read() (map[string]string, error)
}
```

Fueron tres fases de implementación intensiva, con TDD estricto: primero el test (rojo), luego la implementación (verde), luego refactor. 48 tests después, el pipeline de escaneo ya existía como concepto, aunque todavía no había visto una sola IP real.

## El momento de la verdad

Llegó el momento de compilar y probar. Armé el binario, configuré la subred de mi casa (`192.168.1.0/24`), y ejecuté:

```
$ curl http://localhost:8080/api/devices
{"devices":[],"total":0}
```

Silencio. Vacío. Cero dispositivos.

Sabía que mi red no estaba vacía — hay un router, una impresora, varios equipos. ¿Qué había salido mal?

## Tras las pistas

El primer indicio llegó al revisar el código del ping. Había una línea que decía:

```go
pinger.SetPrivileged(true)
```

Esto le dice al sistema que use sockets raw para enviar ICMP, lo que requiere `CAP_NET_RAW`, un permiso especial que normalmente solo tiene el usuario root o los contenedores con capacidades extendidas. Sin ese permiso, los pings simplemente no se enviaban, y sin pings no hay dispositivos vivos.

Lo curioso es que el comando `ping` del sistema funcionaba perfectamente sin sudo. Los kernels modernos de Linux soportan ICMP no privilegiado usando un mecanismo diferente (UDP a ciertos puertos que el kernel redirige al manejo de ICMP). Pero la librería Go que estaba usando no lo sabía.

## La solución: adaptarse al entorno

En lugar de forzar un modo u otro, opté por algo más inteligente: al arrancar, el programa intenta abrir un socket ICMP raw. Si lo consigue, usa el modo privilegiado. Si recibe un permiso denegado, usa el modo no privilegiado. Todo automático.

```go
func detectPrivilegedPing() bool {
    conn, err := net.Dial("ip4:icmp", "127.0.0.1")
    if err != nil {
        return false
    }
    conn.Close()
    return true
}
```

La salida pasó de ser un críptico error de permisos a una línea informativa:

```
Ping mode: unprivileged (UDP ICMP)
```

Elegante. Funciona en cualquier lado sin configuración manual.

## El momento de gloria

Segundo intento. Todo compilado, el auto-detect funcionando. Los dedos cruzados:

```json
{
    "devices": [
        {
            "ip": "192.168.1.1",
            "mac": "2c:96:82:75:69:e8",
            "hostname": "_gateway",
            "vendor": "MitraStar Technology Corp."
        },
        {
            "ip": "192.168.1.88",
            "mac": "a4:d7:3c:10:8a:01",
            "vendor": "Seiko Epson Corporation"
        },
        {
            "ip": "192.168.1.104",
            "hostname": "valhalla"
        }
    ],
    "total": 11
}
```

Once dispositivos. Once. Con nombres de fabricante reales: la impresora Epson, el router MitraStar, los switches Commscope, una placa MSI. Once pequeñas victorias verdes en mi terminal.

## La base de datos de fabricantes

Hablando de fabricantes: al principio tenía una base de datos OUI con exactamente 10 entradas de ejemplo. Cisco, Apple, Intel, Synology. Suficiente para pasar los tests, inútil para la vida real.

Reemplacé ese archivo de juguete con la base de datos oficial de la IEEE: 39.481 entradas que cubren prácticamente cualquier fabricante de dispositivos de red del planeta. Todo embejido dentro del binario con `go:embed`, sin depender de servicios externos ni descargas en tiempo de ejecución.

Pasar de 10 a 39.481 entradas fue tan simple como:

```
$ curl -O https://standards-oui.ieee.org/oui/oui.csv
$ awk -F',' '{...}' oui.csv > oui.csv
```

Una línea de bash y un archivo de 4MB después, el reconocimiento de fabricantes pasó de ser una función decorativa a una herramienta genuinamente útil.

## Docker, CI, y el bonus de mDNS

Con la aplicación funcionando localmente, el siguiente paso fue prepararla para producción. El Dockerfile usa construcción multi-etapa con Alpine. El binario final pesa menos de 10MB y corre como un usuario no root con capacidades restringidas.

El pipeline de CI en GitHub Actions ejecuta build, vet, y tests en dos configuraciones: con y sin el tag de mDNS. Porque sí, también hay una implementación opcional de descubrimiento por mDNS, activable mediante una variable de entorno, para esos casos donde los nombres `.local` marcan la diferencia.

## Conclusión

Construir netwatch fue un recordatorio de que el software no funciona hasta que toca el mundo real. Los tests pasaban, los tipos eran correctos, la arquitectura era limpia. Pero hizo falta ejecutarlo en una red real para descubrir que faltaba un detalle: el modo privilegiado del ping.

Ese es el momento que ninguna prueba unitaria puede reemplazar. Cuando el JSON aparece en la terminal con los dispositivos de tu propia red, con sus nombres y fabricantes reales, y sabes que cada uno de esos 11 dispositivos pasó por el pipeline que construiste — ICMP, ARP, DNS, OUI — y salió del otro lado como un objeto JSON bien formado.

Once dispositivos. Once historias. Un solo API.
