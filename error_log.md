# LOG DE ERRORES

## Problema de inconsistencia en tablas de balances

1. El servicio de ingresos (income_management) utiliza las tablas *_cash_bank_balance
2. Los servicios de gastos (expense_management) y facturas (bills_management) utilizan las tablas originales *_balance
3. Esto causa que los datos se almacenen en lugares diferentes y no se reflejen correctamente

## Evidencia

- Los ingresos se almacenan en monthly_cash_bank_balance pero no en monthly_balance
- Los gastos y facturas intentan actualizar monthly_balance pero no monthly_cash_bank_balance
- No hay sincronización entre ambos conjuntos de tablas
