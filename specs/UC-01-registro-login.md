# UC-01 — Registro e inicio de sesión

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Permitir que una persona cree su cuenta e inicie sesión para acceder a la plataforma, y que el administrador gestione los roles de los usuarios.

## Reglas de negocio
- BR-10: sin sesión válida solo son accesibles el registro y el inicio de sesión; el acceso denegado no produce efectos.
- BR-12: solo `admin` gestiona usuarios (incluida la asignación de rol).

## Criterios de aceptación
- **UC-01.1** Dado un visitante sin sesión, cuando se registra con nombre, email no utilizado y contraseña, entonces se crea un usuario con rol `ingeniero` y la respuesta no incluye `password_hash` (PA-06).
- **UC-01.2** Dado un email ya registrado, cuando otro visitante intenta registrarse con ese email, entonces el registro es rechazado con un error de validación y no se crea un segundo usuario.
- **UC-01.3** Dado un usuario registrado, cuando inicia sesión con credenciales correctas, entonces obtiene una sesión válida y accede a la aplicación.
- **UC-01.4** Dado un usuario registrado, cuando inicia sesión con una contraseña incorrecta, entonces el acceso es rechazado y no se crea sesión.
- **UC-01.5** Dado un visitante sin sesión, cuando solicita cualquier recurso distinto de registro e inicio de sesión, entonces recibe una respuesta de no autenticado y no se produce ningún efecto (BR-10).
- **UC-01.6** Dado un `admin` autenticado, cuando cambia el rol de un usuario a `oncall` o `admin`, entonces el cambio queda guardado; si lo intenta un `ingeniero` o un `oncall`, entonces la acción es prohibida y el rol no cambia (BR-12, BR-10, PA-06).

## Fuera de alcance
- Recuperación o cambio de contraseña.
- Inicio de sesión con proveedores externos (OAuth, SSO).
- Verificación de email y autenticación de dos factores.
- Siembra del primer `admin` por entorno (pendiente, PA-06).

## Verificación
- Unit: `TestBR10_RutasProtegidasSinSesion`, `TestBR12_SoloAdminCambiaRol`, `TestRegistro_EmailDuplicadoRechazado`, `TestRegistro_RolInicialIngeniero` en `backend/internal/auth/...` y `backend/internal/user/...`
- E2E: `e2e/uc-01.spec.ts` (un test por criterio de aceptación)
