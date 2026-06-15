import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
  createRootRoute,
  createRoute,
  Outlet,
} from "@tanstack/react-router";
import { EditQuotationPage } from "@/routes/_authenticated/quotations/$id.edit";
import { NewQuotationPage } from "@/routes/_authenticated/quotations/new";
import {
  resetMswState,
  seedCustomer,
  seedMadridPricingEntry,
  seedPricingEntry,
  seedQuotationDraft,
  seedQuotationSubmitted,
  seedSingleClassPricingEntry,
} from "@/test-utils/msw/handlers";
import { worker } from "@/test-utils/msw/server";
import { http, HttpResponse } from "msw";
import { Toaster } from "sonner";
import {
  describe,
  it,
  expect,
  beforeAll,
  beforeEach,
  afterAll,
  vi,
} from "vitest";
import { render } from "vitest-browser-react";
import { userEvent } from "vitest/browser";
import { useAuthStore } from "@/stores/auth-store";
import { __resetAuthInterceptorState } from "@/lib/api";
import { SidebarProvider } from "@/components/ui/sidebar";
import { Quotations } from "@/features/quotation";
import { QuotationExportActions } from "@/features/quotation/components/quotation-export-actions";
import { QuotationSnapshotView } from "@/features/quotation/components/quotation-snapshot";
import { QuotationDetail } from "@/features/quotation/detail";
import type { Quotation } from "@/features/quotation/types";
import { __resetWizardStorePool } from "@/features/quotation/wizard/quotation-wizard";

// Fixed IDs so seed + router + assertions line up.
const ADMIN_ID = "00000000-0000-0000-0000-000000000001";
const COUNTRY_CN_ID = "00000000-0000-0000-0000-000000000100";
const COUNTRY_US_ID = "00000000-0000-0000-0000-000000000101";
const COUNTRY_AR_ID = "00000000-0000-0000-0000-000000000102";

function buildRouter(
  role: "admin" | "salesperson" | "reviewer",
  initialPath: string,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const user = {
    id: ADMIN_ID,
    name: "Bootstrap Admin",
    email: "admin@example.com",
    phone: "",
    role,
    status: "active" as const,
  };
  queryClient.setQueryData(["auth", "me"], user);
  useAuthStore.getState().auth.setUser(user);

  const rootRoute = createRootRoute({
    component: () => (
      <SidebarProvider>
        <Outlet />
        <Toaster />
      </SidebarProvider>
    ),
  });
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/quotations",
    validateSearch: (s: Record<string, unknown>) => ({
      status: s.status as string | undefined,
    }),
    component: Quotations,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/quotations/$id",
    component: QuotationDetail,
  });
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/quotations/new",
    component: NewQuotationPage,
  });
  const editRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/quotations/$id/edit",
    component: EditQuotationPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      listRoute,
      detailRoute,
      newRoute,
      editRoute,
    ]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { queryClient },
  });
  return { router, queryClient };
}

describe("quotation integration", () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: "bypass" });
  });
  beforeEach(() => {
    resetMswState();
    __resetAuthInterceptorState();
    useAuthStore.getState().auth.reset();
  });
  afterAll(() => {
    worker.stop();
  });

  it("renders empty list state when no quotations exist", async () => {
    const { router, queryClient } = buildRouter("admin", "/quotations");
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    await expect
      .element(screen.getByRole("heading", { name: "报价列表" }))
      .toBeInTheDocument();
    await expect.element(screen.getByText(/暂无报价记录/)).toBeInTheDocument();
  });

  it("admin submits a draft → sees frozen snapshot → approves", async () => {
    // Seed backing data: a pricing entry (¥500.00 application fee),
    // a customer, and a draft quotation owned by admin.
    const custId = seedCustomer({ name: "Acme 国际" });
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: "basic",
      fee_item: "application",
      amount_cny_cents: 50000,
    });
    const quoteId = seedQuotationDraft({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      service_tier: "basic",
    });

    const { router, queryClient } = buildRouter(
      "admin",
      `/quotations/${quoteId}`,
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    // Draft view: status badge is "草稿", snapshot hint is shown.
    await expect.element(screen.getByText("草稿").first()).toBeInTheDocument();
    await expect.element(screen.getByText(/草稿尚未提交/)).toBeInTheDocument();

    // Submit triggers MSW /submit which freezes the ¥500.00 line.
    await userEvent.click(screen.getByRole("button", { name: "提交审核" }));

    // After submit: status badge flips to 已提交 and the snapshot
    // table shows the seeded ¥500.00 as a total cell.
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();
    await expect.element(screen.getByText("申请费")).toBeInTheDocument();
    await expect
      .element(screen.getByText("¥500.00").first())
      .toBeInTheDocument();

    // Approve — opens dialog, confirm without comment.
    await userEvent.click(screen.getByRole("button", { name: "通过" }));
    // Dialog's confirm button is also labelled 确认.
    await userEvent.click(screen.getByRole("button", { name: "确认" }));

    // Status badge now reads 已通过.
    await expect
      .element(screen.getByText("已通过").first())
      .toBeInTheDocument();

    // Once approved, the export actions become visible.
    await expect
      .element(screen.getByRole("button", { name: "导出 PDF" }))
      .toBeInTheDocument();
    await expect
      .element(screen.getByRole("button", { name: "导出 Word" }))
      .toBeInTheDocument();
    await expect
      .element(screen.getByRole("button", { name: "导出 Excel" }))
      .toBeInTheDocument();
  });
});

describe("withdraw + copy + adjust", () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: "bypass" });
  });
  beforeEach(() => {
    resetMswState();
    __resetAuthInterceptorState();
    useAuthStore.getState().auth.reset();
  });
  afterAll(() => {
    worker.stop();
  });

  it("salesperson withdraws a submitted quotation back to draft", async () => {
    const custId = seedCustomer({ name: "Acme 国际" });
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
    });

    const { router, queryClient } = buildRouter(
      "salesperson",
      `/quotations/${quoteId}`,
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    // Initial submitted badge.
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();

    // Click withdraw — flips back to draft.
    await userEvent.click(screen.getByRole("button", { name: "撤回草稿" }));

    await expect.element(screen.getByText("草稿").first()).toBeInTheDocument();
    await expect
      .element(screen.getByText("报价已撤回为草稿"))
      .toBeInTheDocument();
  });

  it("reviewer adjusts a submitted snapshot and sees diff in history", async () => {
    const custId = seedCustomer({ name: "Acme 国际" });
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      total_cny_cents: 10000,
    });

    const { router, queryClient } = buildRouter(
      "reviewer",
      `/quotations/${quoteId}`,
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    // Make sure the submitted snapshot rendered before interacting.
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();
    await expect.element(screen.getByText("申请费")).toBeInTheDocument();

    // Open the adjust sheet.
    await userEvent.click(screen.getByRole("button", { name: "调价" }));

    // The sheet contains one spinbutton (the amount input). Replace it.
    const amountInput = screen.getByRole("spinbutton").first();
    await userEvent.fill(amountInput, "15000");

    // Save the adjustment.
    await userEvent.click(screen.getByRole("button", { name: "保存" }));

    // Confirmation toast + new diff row in the history timeline.
    await expect.element(screen.getByText("调价已保存")).toBeInTheDocument();
    await expect
      .element(screen.getByText(/¥100\.00 → ¥150\.00/))
      .toBeInTheDocument();
  });

  it("copy lands on detail page of the new draft", async () => {
    const custId = seedCustomer({ name: "Acme 国际" });
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
    });

    const { router, queryClient } = buildRouter(
      "admin",
      `/quotations/${quoteId}`,
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    // Source record shows 已提交 first.
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();

    // Click the copy button.
    await userEvent.click(screen.getByRole("button", { name: "复制报价" }));

    // Router should navigate to the new draft; detail page shows 草稿 badge.
    await expect.element(screen.getByText("草稿").first()).toBeInTheDocument();
    await expect
      .element(screen.getByText("报价已复制为新草稿"))
      .toBeInTheDocument();
  });
});

describe("QuotationExportActions", () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: "bypass" });
  });
  beforeEach(() => {
    resetMswState();
    __resetAuthInterceptorState();
    useAuthStore.getState().auth.reset();
  });
  afterAll(() => {
    worker.stop();
  });

  it("calls window.open with signed download_url after clicking PDF bilingual", async () => {
    const QUOTATION_ID = "test-quotation-export-1";
    const DOWNLOAD_URL = "/api/v1/exports/exp-1/download?token=abc";

    // Capture the request body sent to the export endpoint.
    let capturedBody: unknown = null;

    worker.use(
      http.post(
        `/api/v1/quotations/${QUOTATION_ID}/export`,
        async ({ request }) => {
          capturedBody = await request.json();
          const now = new Date().toISOString();
          const dto = {
            id: "exp-1",
            quotation_id: QUOTATION_ID,
            format: "pdf",
            language: "bilingual",
            sha256: "abc123",
            file_size: 1024,
            expires_at: now,
            created_at: now,
            download_url: DOWNLOAD_URL,
          };
          return HttpResponse.json(dto, { status: 201 });
        },
      ),
    );

    // Spy on window.open before rendering.
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    const approvedQuotation: Quotation = {
      id: QUOTATION_ID,
      customer_id: "cust-1",
      country_id: "country-1",
      service_tier: "basic",
      status: "approved",
      created_by: "user-1",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <QuotationExportActions quotation={approvedQuotation} />
        <Toaster />
      </QueryClientProvider>,
    );

    // Click the PDF dropdown trigger button.
    await userEvent.click(screen.getByRole("button", { name: "导出 PDF" }));

    // Click the bilingual menu item.
    await userEvent.click(screen.getByText("中英双语"));

    // Wait for mutation to settle by checking window.open was called.
    await expect.poll(() => openSpy.mock.calls.length).toBeGreaterThan(0);

    expect(openSpy).toHaveBeenCalledWith(DOWNLOAD_URL, "_blank", "noopener");
    expect(capturedBody).toEqual({ format: "pdf", language: "bilingual" });

    openSpy.mockRestore();
  });
});

describe("QuotationSnapshotView", () => {
  it("renders method fee items in Chinese labels", async () => {
    const screen = await render(
      <QuotationSnapshotView
        snapshot={{
          lines: [
            {
              fee_item: "Madrid base official fee",
              amount_cny_cents: 574640,
            },
            {
              fee_item: "Single filing first class fee",
              amount_cny_cents: 790000,
            },
          ],
          total_cny_cents: 1364640,
          signature: "sig-abc",
        }}
      />,
    );

    await expect
      .element(screen.getByText("马德里基础官费"))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("单一注册首类费"))
      .toBeInTheDocument();
  });
});

describe("quotation form", () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: "bypass" });
  });
  beforeEach(() => {
    resetMswState();
    __resetAuthInterceptorState();
    useAuthStore.getState().auth.reset();
    localStorage.clear();
    __resetWizardStorePool();
  });
  afterAll(() => {
    worker.stop();
  });

  async function seedWizardPrereqs() {
    const custId = seedCustomer({ name: "Acme 国际" });
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: "basic",
      fee_item: "application",
      amount_cny_cents: 50000,
    });
    return { custId };
  }

  async function fillRequiredQuotationForm(
    screen: Awaited<ReturnType<typeof render>>,
  ) {
    await userEvent.click(screen.getByRole("combobox", { name: /客户/ }));
    await userEvent.click(screen.getByRole("option", { name: /Acme/ }));
    await selectNiceClass(screen, "9", /第 9 类/);
    await selectSingleCountry(screen, "CN", /中国/);
  }

  async function openNiceClassesSelect(
    screen: Awaited<ReturnType<typeof render>>,
  ) {
    await userEvent.click(screen.getByRole("combobox", { name: /商标类别/ }));
  }

  async function searchNiceClasses(
    screen: Awaited<ReturnType<typeof render>>,
    query: string,
  ) {
    await userEvent.fill(screen.getByPlaceholder("按类别编号或名称搜索"), query);
  }

  async function selectNiceClass(
    screen: Awaited<ReturnType<typeof render>>,
    query: string,
    name: RegExp,
  ) {
    await openNiceClassesSelect(screen);
    await searchNiceClasses(screen, query);
    await userEvent.click(screen.getByRole("checkbox", { name }));
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  }

  async function openSingleCountriesSelect(
    screen: Awaited<ReturnType<typeof render>>,
  ) {
    await userEvent.click(screen.getByRole("combobox", { name: /单一注册/ }));
  }

  async function searchCountries(
    screen: Awaited<ReturnType<typeof render>>,
    query: string,
  ) {
    await userEvent.fill(screen.getByPlaceholder("按国家代码或名称搜索"), query);
  }

  async function selectSingleCountry(
    screen: Awaited<ReturnType<typeof render>>,
    query: string,
    name: RegExp,
  ) {
    await openSingleCountriesSelect(screen);
    await searchCountries(screen, query);
    await userEvent.click(screen.getByRole("checkbox", { name }));
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  }

  async function openMadridCountriesSelect(
    screen: Awaited<ReturnType<typeof render>>,
  ) {
    await userEvent.click(screen.getByRole("combobox", { name: /马德里注册/ }));
  }

  async function selectMadridCountry(
    screen: Awaited<ReturnType<typeof render>>,
    query: string,
    name: RegExp,
  ) {
    await openMadridCountriesSelect(screen);
    await searchCountries(screen, query);
    await userEvent.click(screen.getByRole("checkbox", { name }));
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  }

  it("new form → save draft → detail keeps extended fields", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await fillRequiredQuotationForm(screen);

    await expect.element(screen.getByText(/申请费/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "保存草稿" }));

    await expect.element(screen.getByText("草稿").first()).toBeInTheDocument();
    await expect
      .element(screen.getByText(/第 9 类 科学仪器/))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("单一注册：中国（CN）"))
      .toBeInTheDocument();
    await expect.element(screen.getByText(/A 代理/)).toBeInTheDocument();
  });

  it("new form → separate madrid and single countries keep pricing and detail grouping", async () => {
    seedCustomer({ name: "Acme 国际" });
    seedMadridPricingEntry({
      country_area: "Base fee",
      official_fee_chf_cents: 65300,
      agency_fee_cny_cents: 120000,
      is_base_fee: true,
    });
    seedMadridPricingEntry({
      country_id: COUNTRY_US_ID,
      country_area: "United States",
      official_fee_chf_cents: 12000,
      agency_fee_cny_cents: 30000,
      is_base_fee: false,
    });
    seedSingleClassPricingEntry({
      country_id: COUNTRY_AR_ID,
      continent: "South America",
      country_area: "Argentina",
      first_class_fee_cny_cents: 280000,
      additional_class_fee_cny_cents: 60000,
    });

    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await userEvent.click(screen.getByRole("combobox", { name: /客户/ }));
    await userEvent.click(screen.getByRole("option", { name: /Acme/ }));
    await selectNiceClass(screen, "9", /第 9 类/);
    await selectMadridCountry(screen, "US", /美国/);
    await selectSingleCountry(screen, "AR", /阿根廷/);

    await expect
      .element(screen.getByText("马德里基础官费"))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("马德里指定国家官费"))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("单一注册首类费"))
      .toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "保存草稿" }));

    await expect.element(screen.getByText("草稿").first()).toBeInTheDocument();
    await expect
      .element(screen.getByText("马德里注册：美国（US）"))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("单一注册：阿根廷（AR）"))
      .toBeInTheDocument();
    await expect
      .element(screen.getByText("国家：美国（US）、阿根廷（AR）"))
      .toBeInTheDocument();
  });

  it("new form → save and submit → status becomes submitted", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await fillRequiredQuotationForm(screen);

    await expect.element(screen.getByText(/申请费/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "保存并提交" }));
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();
  });

  it("edit → change agent level → save and submit → status=submitted", async () => {
    const { custId } = await seedWizardPrereqs();
    const draftId = seedQuotationDraft({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      service_tier: "basic",
    });
    // Add pricing for B-agent tier so the post-edit preview finds
    // matching pricing entries and submit doesn't fail.
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: "standard",
      fee_item: "application",
      amount_cny_cents: 80000,
    });

    const { router, queryClient } = buildRouter(
      "salesperson",
      `/quotations/${draftId}/edit`,
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await expect
      .element(screen.getByRole("combobox", { name: /中国/ }))
      .toBeInTheDocument();
    await selectNiceClass(screen, "9", /第 9 类/);
    await userEvent.click(screen.getByRole("radio", { name: /B 代理/ }));
    await expect
      .element(screen.getByRole("radio", { name: /B 代理/ }))
      .toBeChecked();
    await expect.element(screen.getByText(/申请费/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "保存并提交" }));
    await expect
      .element(screen.getByText("已提交").first())
      .toBeInTheDocument();
  });

  it("resume banner: pre-seeded localStorage → banner shows → discard clears form", async () => {
    await seedWizardPrereqs();
    localStorage.setItem(
      `quotation-wizard-draft:${ADMIN_ID}`,
      JSON.stringify({
        state: {
          currentStep: 2,
          editingId: null,
          draft: {
            customer_id: "stale-customer",
            country_ids: [COUNTRY_CN_ID],
            nice_category_codes: [9],
            registration_methods: ["single"],
            agent_level: "agent_b",
            info_sections: [],
            notes: "stale notes",
          },
        },
        version: 0,
      }),
    );

    const { router, queryClient } = buildRouter("admin", "/quotations/new");
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await expect.element(screen.getByText(/未完成的草稿/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /放弃/ }));
    await expect
      .element(screen.getByText(/未完成的草稿/))
      .not.toBeInTheDocument();
  });

  it("preview error: ERR_MISSING_PRICING → retry button + both saves disabled", async () => {
    seedCustomer({ name: "Acme 国际" });

    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await fillRequiredQuotationForm(screen);

    await expect.element(screen.getByText(/暂无定价/)).toBeInTheDocument();
    await expect
      .element(screen.getByRole("button", { name: "保存草稿" }))
      .toBeDisabled();
    await expect
      .element(screen.getByRole("button", { name: "保存并提交" }))
      .toBeDisabled();
    await expect
      .element(screen.getByRole("button", { name: /重试/ }))
      .toBeInTheDocument();
  });

  it("nice classes search narrows options and shows the selected summary", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await openNiceClassesSelect(screen);
    await searchNiceClasses(screen, "35");

    await expect
      .element(screen.getByRole("checkbox", { name: /第 35 类/ }))
      .toBeInTheDocument();
    await expect
      .element(screen.getByRole("checkbox", { name: /第 9 类/ }))
      .not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("checkbox", { name: /第 35 类/ }));
    await expect
      .element(screen.getByRole("combobox", { name: /第 35 类/ }))
      .toBeInTheDocument();
  });

  it("nice classes clear removes selections and disables saving", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await fillRequiredQuotationForm(screen);
    await expect.element(screen.getByText(/申请费/)).toBeInTheDocument();

    await openNiceClassesSelect(screen);
    await userEvent.click(screen.getByRole("button", { name: "清空" }));

    await expect
      .element(screen.getByRole("button", { name: "保存草稿" }))
      .toBeDisabled();
    await expect
      .element(screen.getByRole("button", { name: "保存并提交" }))
      .toBeDisabled();
    await expect
      .element(screen.getByRole("combobox", { name: /请选择商标类别/ }))
      .toBeInTheDocument();
  });

  it("nice classes select all only toggles the filtered result set", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await openNiceClassesSelect(screen);
    await searchNiceClasses(screen, "35");
    await userEvent.click(screen.getByRole("button", { name: "全选" }));

    await expect
      .element(screen.getByRole("combobox", { name: /第 35 类/ }))
      .toBeInTheDocument();
    await expect
      .element(screen.getByRole("combobox", { name: /第 9 类/ }))
      .not.toBeInTheDocument();
  });

  it("countries search narrows options and shows the selected summary", async () => {
    await seedWizardPrereqs();
    const { router, queryClient } = buildRouter(
      "salesperson",
      "/quotations/new",
    );
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    await openSingleCountriesSelect(screen);
    await searchCountries(screen, "US");

    await expect
      .element(screen.getByRole("checkbox", { name: /美国/ }))
      .toBeInTheDocument();
    await expect
      .element(screen.getByRole("checkbox", { name: /中国/ }))
      .not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("checkbox", { name: /美国/ }));
    await expect
      .element(screen.getByRole("combobox", { name: /美国/ }))
      .toBeInTheDocument();
  });
});
