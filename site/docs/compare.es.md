# Comparación

Hay muy buenas herramientas para desplegar en un VPS. Esta página explica en qué se parece y en qué se diferencia versaDeploy, para que elijas la que mejor te sirve. Si ves algo desactualizado, [avísanos](https://github.com/kriollo/versaDeploy/issues).

| | **versaDeploy** | Laravel Forge + Envoyer | Deployer (PHP) | Kamal | Coolify / Dokploy |
| --- | --- | --- | --- | --- | --- |
| **Precio** | Gratis, MIT | Suscripción mensual | Gratis, MIT | Gratis, MIT | Gratis autoalojado; versión cloud de pago |
| **Desde dónde** | Tu computador o tu CI | Panel web (SaaS) | Tu computador o tu CI | Tu computador o tu CI | Panel web instalado en tu servidor |
| **Qué necesita el servidor** | Solo SSH | SSH (lo administra Forge) | Solo SSH | Docker | Docker + la plataforma |
| **Agente en el servidor** | No | No | No | No | Sí (la plataforma) |
| **Dónde compila** | En tu computador; sube solo lo que cambió | En el servidor | En el servidor | Construye una imagen Docker | En el servidor |
| **Releases atómicos y rollback** | Sí, automático si falla | Sí (Envoyer) | Sí | Sí (contenedores) | Sí (contenedores) |
| **Prepara el servidor** | Sí (vista Services) | Sí (Forge) | Parcial | Instala Docker | Sí |
| **Lenguajes** | PHP, Node, Python, Go | Sobre todo PHP | Sobre todo PHP | Cualquiera, en contenedores | Cualquiera, en contenedores |
| **Interfaz** | CLI + TUI | Web | CLI | CLI | Web |

## Cuándo elegir versaDeploy

- Quieres **desplegar en tu propio VPS sin Docker y sin un panel** corriendo en él: el servidor solo necesita SSH.
- Tu servidor es **pequeño o antiguo**: compilar en tu computador ahorra CPU y RAM al servidor, y los deploys funcionan incluso en CentOS 5.
- Quieres que alguien **prepare el servidor** (nginx, PHP, base de datos, SSL, fail2ban, backups) **sin pagar una suscripción**, y que respete lo que ya está instalado.
- Tienes **apps en varios lenguajes** (Laravel, Nuxt, FastAPI, Go) y quieres el mismo flujo para todas.

## Cuándo elegir otra

- **Forge + Envoyer**: prefieres un panel web administrado con soporte, y el costo mensual no es problema.
- **Deployer**: tu equipo ya es PHP, te gustan las recetas en PHP y no necesitas que la herramienta prepare el servidor.
- **Kamal**: ya trabajas con Docker y quieres desplegar imágenes en tus servidores.
- **Coolify / Dokploy**: quieres una "plataforma tipo Heroku" en tu servidor, con panel web, bases de datos administradas y despliegues desde git, y no te molesta correr Docker y la plataforma en él.

!!! note
    Esta tabla resume cada herramienta a grandes rasgos; todas evolucionan rápido. Revisa su documentación para los detalles.
