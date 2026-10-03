# KubeTray

Icono de bandeja para cambiar el contexto actual de Kubernetes desde la
barra de tu escritorio. Sin GTK: el icono y el menú hablan directamente
con las interfaces D-Bus StatusNotifierItem y com.canonical.dbusmenu de
freedesktop, así que las únicas dependencias en tiempo de ejecución son
D-Bus y `kubectl`.

---

## Características

- **Menú de contextos**: lista todos los contextos de
  `kubectl config get-contexts`, con una marca de radio en el actual. Los
  contextos de GKE (`gke_<proyecto>_<ubicación>_<cluster>`) se agrupan bajo
  su proyecto y se muestran como `cluster · ubicación`; el resto va bajo
  "Other".
- **Cambio con un clic**: al hacer clic en un contexto se ejecuta
  `kubectl config use-context <nombre>`.
- **Actualización en vivo**: los archivos de kubeconfig (`$KUBECONFIG`, o
  `~/.kube/config`) se revisan cada 2s, así que un
  `kubectl config use-context` desde una terminal, o clusters nuevos,
  aparecen sin reiniciar la app.
- **Aviso de producción**: el tooltip muestra el cluster y proyecto
  actuales, y el icono se pone rojo cuando el nombre del contexto actual
  contiene `prd`, `prod` o `production` como palabra separada.
- **Avisos de sesión**: si el plugin de credenciales del contexto actual
  necesita una sesión de nube que no está activa, aparece un aviso con el
  comando para arreglarlo debajo de los contextos. Se vuelve a comprobar
  cada minuto.

  | Proveedor | Comprobación |
  | :--- | :--- |
  | GCP | `gcloud auth print-access-token` |
  | AWS | `aws sts get-caller-identity` (con el perfil del plugin) |
  | Azure | `az account get-access-token` (solo en el modo `azurecli` de kubelogin) |

- **Inicio con la sesión**: "Launch at login" activa/desactiva
  `~/.config/autostart/kubetray.desktop`.

---

## Requisitos Previos

- **[Go 1.22+](https://go.dev/dl/)** (para compilar)
- **[`kubectl`](https://kubernetes.io/docs/tasks/tools/)**
- **Un StatusNotifierHost** para mostrar el icono, p. ej. el módulo `tray`
  de waybar, o `snixembed` si tu barra no soporta StatusNotifierItem.
- La CLI de cada nube que uses (`gcloud`, `aws`, `az`) para los avisos de
  sesión.

---

## Instalación

```bash
make build                # genera ./kubetray
sudo make install         # instala en /usr/local/bin (PREFIX=/usr para cambiarlo)
sudo make uninstall
```

Después ejecuta `kubetray` y activa "Launch at login" desde el menú si
quieres que arranque con tu sesión.

---

## Configuración

`kubectl` se busca en el `PATH`, y si no está, en `~/google-cloud-sdk/bin`,
`~/.local/bin`, `/usr/local/bin` y `/usr/bin` (las sesiones de autostart no
suelen heredar el `PATH` de tu shell). Usa `$KUBECTL` para forzar una ruta.

El tamaño del icono lo decide tu tray host (p. ej. `icon-size` en waybar);
KubeTray entrega el icono en varios tamaños para que elija.

---

## Script auxiliar: `sync-gke.sh`

Script de apoyo para cargar en tu `kubeconfig` los clusters de **GKE**.
Recorre todos los proyectos de Google Cloud de tu organización o cuenta,
descubre los clusters y registra sus credenciales, omitiendo los contextos
que ya existen. KubeTray detecta los contextos nuevos automáticamente.

> Quedan pendientes scripts equivalentes para **AWS (EKS)** y **Azure (AKS)**.

**Requisitos:** [`gcloud`](https://cloud.google.com/sdk/docs/install),
`kubectl`, Bash 4.0+ y
[`gke-gcloud-auth-plugin`](https://cloud.google.com/blog/products/containers-kubernetes/introducing-gke-gcloud-auth-plugin)
(`gcloud components install gke-gcloud-auth-plugin`).

**Qué hace:**

- Pide `gcloud auth login` si no hay una sesión activa.
- Ignora los proyectos de sistema (`sys-*`).
- Verifica si `gke_<PROYECTO>_<UBICACIÓN>_<CLUSTER>` ya existe antes de
  pedir las credenciales.
- Sigue adelante si un proyecto no tiene la API de Kubernetes Engine
  habilitada o no hay permisos.
- Muestra un resumen con proyectos escaneados, clusters encontrados,
  agregados y omitidos.

**Uso:**

```bash
chmod +x sync-gke.sh
./sync-gke.sh             # sincronización estándar
./sync-gke.sh --dry-run   # previsualiza sin tocar kubeconfig
./sync-gke.sh --force     # renueva credenciales de contextos existentes
```

| Opción | Descripción |
| :--- | :--- |
| `-d`, `--dry-run` | Muestra qué clusters se añadirían sin modificar `kubeconfig`. |
| `-f`, `--force` | Obtiene las credenciales aunque el contexto ya exista. |
| `-i`, `--internal-ip` | Agrega los clusters usando su endpoint privado. |
| `-v`, `--verbose` | Muestra mensajes detallados. |
| `-h`, `--help` | Muestra el menú de ayuda. |

---

## 📄 Licencia

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
