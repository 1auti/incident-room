# UC-01 — Registro e inicio de sesión

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Permitir que una persona cree su cuenta e inicie sesión para acceder a la plataforma, y que el administrador gestione los roles de los usuarios.

## Reglas de negocio
- BR-10: sin sesión válida solo son accesibles el registro y el inicio de sesión; el acceso denegado no produce efectos.
- BR-12: solo `admin` gestiona usuarios (incluida la asignación de rol).
- BR-19: el registro público crea usuarios `ingeniero` con email único; solo el `admin` cambia el rol; el primer `admin` se crea al iniciar la aplicación con credenciales de variables de entorno.

## Criterios de aceptación
- **UC-01.1** Dado un visitante sin sesión, cuando se registra con nombre, email no utilizado y contraseña, entonces se crea un usuario con rol `ingeniero` y la respuesta no incluye `password_hash` (BR-19).
- **UC-01.2** Dado un email ya registrado, cuando otro visitante intenta registrarse con ese email, entonces el registro es rechazado con un error de validación y no se crea un segundo usuario (BR-19).
- **UC-01.3** Dado un usuario registrado, cuando inicia sesión con credenciales correctas, entonces obtiene una sesión válida y accede a la aplicación; con una contraseña incorrecta, el acceso es rechazado y no se crea sesión.
- **UC-01.4** Dado un visitante sin sesión, cuando solicita cualquier recurso distinto de registro e inicio de sesión, entonces recibe una respuesta de no autenticado y no se produce ningún efecto (BR-10).
- **UC-01.5** Dado un `admin` autenticado, cuando cambia el rol de un usuario a `oncall` o `admin`, entonces el cambio queda guardado; si lo intenta un `ingeniero` o un `oncall`, entonces la acción es prohibida y el rol no cambia (BR-12, BR-10, BR-19).
- **UC-01.6** Dado que no existe ningún `admin` y hay credenciales de administrador en variables de entorno, cuando la aplicación inicia, entonces se crea un `admin` con esas credenciales y puede iniciar sesión; si ya existe un `admin`, no se crea otro (BR-19).

## Fuera de alcance
- Recuperación o cambio de contraseña.
- Inicio de sesión con proveedores externos (OAuth, SSO).
- Verificación de email y autenticación de dos factores.
- Cualquier otro modo de obtener el primer `admin`: el registro público siempre crea `ingeniero` (BR-19).

## Verificación
- Unit: `TestBR10_RutasProtegidasSinSesion`, `TestBR12_SoloAdminCambiaRol`, `TestBR19_RegistroCreaIngeniero`, `TestBR19_EmailUnico`, `TestBR19_PrimerAdminDesdeEntorno` en `backend/internal/auth/...` y `backend/internal/user/...`
- E2E: `e2e/uc-01.spec.ts` (un test por criterio de aceptación)
