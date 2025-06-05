# PLAN DE ACCIÓN PARA RESOLVER INCONSISTENCIA DE TABLAS

## Opciones de solución

### Opción 1: Actualizar gastos y facturas para usar las tablas cash_bank

1. Modificar updateMonthlyBalance en expense_management para usar monthly_cash_bank_balance
2. Modificar updateMonthlyBalance en bills_management para usar monthly_cash_bank_balance
3. Modificar todas las funciones relacionadas con balances en ambos servicios
4. Crear un script de migración para mover datos existentes

### Opción 2: Actualizar ingresos para usar las tablas originales

1. Modificar updateMonthlyBalance en income_management para usar monthly_balance
2. Convertir funciones y migrar datos

### Opción 3: Solución híbrida

1. Crear un nuevo servicio de sincronización que mantenga ambos conjuntos de tablas actualizados
2. Añadir triggers o procedimientos almacenados para mantener sincronización

## Recomendación

Recomendamos la Opción 1 porque:

1. Las tablas cash_bank parecen ser más recientes y mejor diseñadas
2. Tienen campos específicos para efectivo y banco que son más detallados
3. El servicio de ingresos ya está funcionando correctamente con ellas
