# Un nuevo comienzo

## El problema

Siempre me ha dado curiosidad saber qué dispositivos están realmente conectados a mi red local. El router muestra algunos nombres, otros aparecen como direcciones IP sin rostro, y algunos simplemente se esconden. Herramientas como `nmap` existen, pero son pesadas para un uso cotidiano y no exponen una API que pueda consumir desde un front-end liviano.

Así nace **netwatch**: un descubridor de dispositivos de red, liviano, que corre en un contenedor Docker y expone una API REST.

## La arquitectura

### Lenguaje y stack

Go fue la elección natural para esto: compilación cruzada sencilla, imágenes Docker pequeñas (5-8MB con Alpine), concurrencia nativa para los barridos de red, y una stdlib que cubre la mayoría de las necesidades.

El proyecto se organiza en torno a cuatro paquetes internos claramente separados:

```
cmd/api/             → Punto de entrada
internal/config/     → Configuración por variables de entorno
internal/models/     → Tipos compartidos (Device, ScanResult)
internal/scanner/    → Lógica de descubrimiento de red
internal/api/        → Handlers HTTP
```

### Pipeline de escaneo

El proceso de descubrimiento sigue cuatro fases secuenciales:

1. **ICMP ping sweep** — se envía un ping a cada IP de la subred configurada para "despertar" la tabla ARP
2. **Lectura de tabla ARP** — se parsea `/proc/net/arp` para obtener las direcciones IP y MAC de los dispositivos que respondieron
3. **Resolución DNS inversa** — se consulta el nombre asociado a cada IP mediante `net.LookupAddr`
4. **Identificación de vendor** — se determina el fabricante de cada dispositivo a partir del prefijo OUI de su dirección MAC

El resultado se devuelve como un array JSON a través de la API.

### Decisiones técnicas

**Router HTTP**: Se eligió `chi` porque se mantiene muy cerca del `net/http` estándar, no impone magia, y escala bien si la API crece. Para un microservicio de este tamaño, `gin` o `echo` agregarían complejidad innecesaria.

**ICMP**: La librería `go-ping/ping` (pro-bing) es la misma que usa el ecosistema Prometheus. Está activamente mantenida y maneja correctamente los permisos de `CAP_NET_RAW`.

**ARP**: Zero dependencias externas. El archivo `/proc/net/arp` es texto plano, y parsearlo son aproximadamente 50 líneas de Go. Suficiente para el caso de uso.

**Identificación por MAC (OUI)**: Se incorpora una base de datos de prefijos OUI de la IEEE directamente en el binario mediante `go:embed`. Ocupa unos pocos megas y permite identificar el fabricante de cada dispositivo sin depender de servicios externos.

**mDNS**: Es una capacidad opcional, activable mediante una build tag. Por defecto no se incluye para mantener la imagen pequeña y el escaneo rápido.

### La API

El diseño es minimalista pero funcional:

```
GET /api/devices   → Ejecuta un escaneo y devuelve los dispositivos encontrados
GET /health        → Health check del servicio
```

Cada dispositivo se representa como:

```json
{
  "ip": "192.168.1.20",
  "hostname": "smart-tv",
  "mac": "00:11:22:aa:bb:cc",
  "vendor": "Samsung Electronics",
  "response_time_ms": 5
}
```

### Despliegue

El contenedor Docker requiere dos cosas para funcionar correctamente:

- `--net=host`: para acceder a la tabla ARP y la red del host
- `CAP_NET_RAW`: para poder enviar paquetes ICMP sin ejecutarse como root

El Dockerfile usa build multi-etapa con Alpine para obtener una imagen final de menos de 10MB.

## Calidad desde el inicio

Cada componente se construye con pruebas primero. La arquitectura está diseñada para ser testeable: el scanner expone una interfaz que puede ser reemplazada por un mock en las pruebas HTTP, y cada etapa del pipeline (ICMP, ARP, DNS, OUI) se puede probar de forma independiente.

## Transparencia

El repositorio es público. No contiene direcciones IP reales, credenciales ni datos sensibles de ninguna red. La subred a escanear se configura mediante variable de entorno, y los resultados del escaneo viven solo en memoria durante la respuesta de la API — nunca se persisten.

---

Este es el comienzo de netwatch. El objetivo es mantenerlo simple, enfocado y bien construido. Lo que sigue es implementarlo pieza por pieza, con tests, documentación y el cuidado que merece un proyecto que va a correr en mi propia infraestructura.
