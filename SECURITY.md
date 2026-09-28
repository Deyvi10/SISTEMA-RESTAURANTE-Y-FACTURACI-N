# Seguridad

Si encuentras una vulnerabilidad, **no abras un issue público**. Escribe al responsable del repositorio por un canal privado (GitHub → *Security* → *Report a vulnerability*). Respondemos en menos de 72 horas.

Reglas del proyecto (detalle en `documentacion-proyecto/docs/06-seguridad.md`):

- El certificado `.p12` y su contraseña nunca salen de la nube ni se guardan en el repositorio.
- Nada de secretos, cédulas, correos ni datos personales en código, logs, fixtures o issues.
- Los ganchos (`make hooks`) y el CI (gitleaks) bloquean certificados, llaves y archivos `.env`.
