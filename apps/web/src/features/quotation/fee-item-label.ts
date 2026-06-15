const FEE_ITEM_LABELS: Record<string, string> = {
  application: '申请费',
  agent: '代理费',
  'Madrid base official fee': '马德里基础官费',
  'Madrid base agency fee': '马德里基础代理费',
  'Madrid designated country official fee': '马德里指定国家官费',
  'Madrid designated country agency fee': '马德里指定国家代理费',
  'Single filing first class fee': '单一注册首类费',
  'Single filing additional class fee': '单一注册附加类别费',
}

export function translateFeeItemLabel(feeItem: string) {
  return FEE_ITEM_LABELS[feeItem] ?? feeItem
}
