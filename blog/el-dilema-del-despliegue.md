## El problema de los runners

Cuando empecé con proyectos auto-alojados en el NAS, el camino más obvio era instalar un runner de GitHub Actions por cada repositorio. Un systemd service por proyecto, cada uno consumiendo unos 40MB de RAM en idle, todos preguntando "¿hay algo para hacer?" aunque nadie haya tocado el código en semanas.

Al principio no molesta. Pero cuando tienes tres, cuatro, cinco proyectos, empiezas a preguntarte si tiene sentido tener cinco procesos que no hacen más que esperar.

Sobre todo cuando el NAS no es un servidor de producción con recursos infinitos. Es un disco duro con suerte, corriendo Docker, y cada megabyte cuenta.

## La primera iteración: un runner por proyecto

Eso fue lo primero que hice con media-organizer. Un runner registrado a nivel de repositorio, instalado como servicio systemd, ejecutando los builds localmente. Funcionaba, pero:

- Cada proyecto nuevo implicaba instalar otro runner
- El NAS terminaba con un proceso por proyecto
- Había que mantenerlos actualizados
- El setup de permisos se repetía cada vez (¡y vaya si se iteró en permisos!)

## Repensando el modelo

La pregunta que me hice es sencilla: ¿qué tiene que pasar realmente cuando hago push?

1. El NAS necesita saber que hay código nuevo
2. Necesita bajar ese código
3. Necesita construir la imagen
4. Necesita levantar el contenedor

El runner de GitHub Actions resuelve todo eso, pero no es la única forma. De hecho, es la más pesada.

## Las alternativas que surgieron

Aparecieron varias opciones, cada una con su balance entre simpleza y eficiencia:

**Un runner único a nivel de cuenta** — un solo proceso systemd para todos los proyectos. GitHub permite registrar runners no solo por repositorio, sino también por cuenta personal o por organización. Un solo servicio, una sola configuración, labels para distinguir proyectos. Sigue habiendo un proceso, pero solo uno.

**Un receptor de webhooks liviano** — un mini servidor HTTP (en Go, por ejemplo) que corre en el NAS, escucha eventos de GitHub, y ejecuta el deploy. Sin runner, sin agente, sin polling. Un binario de 8MB que no hace absolutamente nada hasta que alguien le pega. Consumo en idle: prácticamente cero.

**Build en la nube + Watchtower** — GitHub Actions construye las imágenes en sus servidores, las sube a un registro, y Watchtower en el NAS las detecta y las actualiza automáticamente. El NAS no ejecuta builds, solo recibe imágenes ya listas.

**SSH remoto vía Tailscale** — y esta fue la que más me gustó.

## La propuesta sobre la mesa

GitHub Actions se conecta al NAS a través de Tailscale (que ya está instalado), abre una sesión SSH, y ejecuta los comandos de deploy ahí mismo. El runner de GitHub no construye nada — solo orquesta. Todo el trabajo pesado lo hace el NAS, que ya tiene Docker, el código, y todo lo necesario.

La clave del modelo está en la centralización: un solo usuario en el NAS (llamémoslo `deploy`), un solo script que recibe el nombre del proyecto como parámetro, y una sola configuración compartida.

Cada proyecto solo necesita un pequeño archivo YAML que diga:

- Dónde está el repositorio
- En qué branch hay que fijarse
- Cómo se llama la imagen

Y una GitHub Action genérica, idéntica en todos los repos, que se conecta por Tailscale y dice: "ejecuta deploy para proyecto X".

## Lo que falta resolver

El modelo está claro, pero los detalles finos van a definir si es realmente mejor que lo que ya tenemos:

- **Los permisos de Docker**: el usuario `deploy` necesita poder construir imágenes y levantar contenedores. Sin acceso root, sin exponer más de lo necesario. La solución más limpia parece ser un sudo acotado, solo para comandos específicos.
- **Los secretos de Tailscale**: se necesita una OAuth key que permita al runner de GitHub autenticarse en la red interna. Una sola key para todos los proyectos, con expiración automática.
- **El script de deploy**: tiene que ser genérico pero extensible. Que funcione para netwatch, para media-organizer, y para cualquier proyecto futuro sin tener que tocar el NAS cada vez.

## Y después de eso

Una vez que el mecanismo de despliegue esté aceitado, la idea es tener un pipeline completo para netwatch:

1. Push a main
2. GitHub Actions valida (build, vet, tests, lint)
3. GitHub Actions se conecta al NAS por Tailscale
4. El NAS construye la imagen
5. El NAS levanta el contenedor
6. netwatch está disponible en la red local

Todo automático, todo centralizado, cero procesos innecesarios.

O al menos, esa es la idea. Todavía quedan decisiones de implementación, y seguramente algunas iteraciones más hasta que el flujo sea tan natural como hacer `git push` y olvidarse.
