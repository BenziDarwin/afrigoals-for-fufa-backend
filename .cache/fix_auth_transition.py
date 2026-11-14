from pathlib import Path
p=Path('F:/Afrigoals/Afrigoals_ecommerce/lib/auth/client.ts');s=p.read_text();a='''      } else {
        refresh();
      }
    });''';b='''      } else {
        setState({ status: "loading", user: null });
        refresh();
      }
    });''';assert a in s;p.write_text(s.replace(a,b),encoding='utf-8')
