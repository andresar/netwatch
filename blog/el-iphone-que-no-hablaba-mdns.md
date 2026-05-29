# El iPhone que no hablaba mDNS (y el que sí)

## El problema de los manzanitas

Resulta que los dispositivos Apple tienen una particularidad: cuando les preguntás su nombre por mDNS con una consulta PTR inversa, ellos responden... pero no con un registro PTR. Responden con un registro A (o AAAA) cuyo nombre es el `.local` hostname.

Suena a detalle menor. Hasta que te das cuenta de que tu código solo miraba registros PTR. Y entonces todas las manzanas de tu red se vuelven invisibles.

```
¿Quién eres?       →  PTR 192.168.1.168 →  (silencio)
                  ←  A  iPhone-de-Alguien.local → ????
```

Mi implementación original de `matchPTR` era inocente:

```go
if ans.Header.Type != dnsmessage.TypePTR {
    continue
}
```

Lógico, correcto, y completamente inútil contra Apple.

## La caída del dominó

Cuando ejecuté el escaneo mDNS por primera vez vi algo curioso: Katherine (Samsung) y Benjamin aparecían sin hostname. Mi iPhone (`.84`) y la MacBook de Andrés (`.136`) tenían MACs aleatorias, así que esperaba encontrar algo — pero no.

El problema no era que no respondieran. El problema era que mi parser no entendía su respuesta. Los otros dispositivos Android sí devolvían PTR como Dios manda. Apple no.

Así que amplié `matchPTR` para que, cuando no encuentra un PTR, busque registros A o AAAA cuyo nombre contenga `.local` y cuya IP coincida con la que estamos consultando:

```go
for _, ans := range set {
    name := strings.ToLower(ans.Header.Name.String())
    if !strings.Contains(name, ".local") {
        continue
    }
    switch body := ans.Body.(type) {
    case *dnsmessage.AResource:
        ip := net.IP(body.A[:]).String()
        if targetIPs[ip] {
            return ip, strings.TrimSuffix(name, ".")
        }
    case *dnsmessage.AAAAResource:
        // igual con IPv6
    }
}
```

## El momento de la verdad

Con el fix implementado, lancé el escáner sobre mi red:

```json
{
    "devices": [
        { "ip": "192.168.1.149", "hostname": "iPhone-de-Katherine.local" },
        { "ip": "192.168.1.170", "hostname": "iPhone-Benjamin.local" },
        { "ip": "192.168.1.88",  "hostname": "EPSON108A01.local" }
    ],
    "total": 17
}
```

¡Ahí estaban! Katherine y Benjamin ya tenían nombre. La impresora Epson, que siempre fue buena ciudadana, seguía ahí con su `.local`.

Pero mi iPhone (`.84`) seguía mudo. Y la MacBook (`.136`) también.

## Otra manzana podrida

Probé manualmente con Python, enviando consultas mDNS directas, esperando respuestas. Nada. `.84` no respondió. `.136` tampoco.

¿Dormidos? ¿Firewall? ¿Deep sleep? ¿iOS bloqueando multicast en background? Posiblemente todo eso. Katherine y Benjamin tienen sus iPhones desbloqueados y activos en la red. Mi iPhone no tanto — suele estar en reposo profundo.

Hice un experimento: le mandé un ping a la MacBook mientras el escaneo mDNS corría. La MacBook, al recibir el ping, despertó — y entonces sí, respondió al mDNS.

Esto abre una pregunta interesante: ¿debería el pipeline enviar un "wake up" (un ping, un ARP, algo) antes de la fase mDNS para despertar a los dormidos? Por ahora no. La prioridad es rapidez (~5s), y despertar dispositivos llevaría más tiempo. Pero queda anotado.

## Lo que aprendí

1. **Apple juega sucio con mDNS**: no esperes registros PTR de dispositivos Apple. Buscá A/AAAA con `.local` en el nombre.
2. **No todos los que están, hablan**: un dispositivo puede estar en la tabla ARP pero no responder mDNS si está dormido. No es un bug, es feature.
3. **17 dispositivos en ~5 segundos**: el pipeline actual (ICMP → ARP → DNS → mDNS → OUI) escanea toda una subred /24 en menos de lo que tarda en calentar el café.

## El próximo capítulo

Quedan tres dispositivos con MAC aleatoria (local_mac: true) sin identificar. Algunos son sin duda Apple. Otros tal vez no. Y la MacBook de Andrés en `.136` sigue siendo un fantasma — aparece en ARP, responde ping, pero no suelta su nombre.

Para la próxima: una sección de tiempos de conexión, métricas de latencia, y tal vez — solo tal vez — el arte de despertar a los dormidos.
